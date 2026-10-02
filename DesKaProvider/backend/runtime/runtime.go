package runtime

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
 "net/http"
 "reflect"
 "os"
 "sort"
 "strconv"
 "strings"
 "sync"
 "time"

 _ "github.com/jackc/pgx/v5/stdlib"

 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/migrations"
 digiflazz "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/DigiFlazz"
 iak "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/IAK"
	midtrans "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Midtrans"
	xpsindonesia "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/XPSindonesia"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

const (
 defaultStorePath="data/operational-snapshots.json"
 defaultProviderStateStorePath="data/provider-state.json"
 defaultTransactionStorePath="data/provider-transactions.json"
 defaultSyncInterval=30*time.Second
 defaultFailureThreshold=3
 defaultCurrency="IDR"
 defaultPriceListCacheTTL=15*time.Minute
 defaultCatalogStorePath="data/product-catalog.json"
 defaultCatalogSyncStatusStorePath="data/catalog-sync-status.json"
 defaultCatalogSyncInterval=15*time.Minute
 defaultCatalogMaxAge=30*time.Minute
 defaultOperationalSnapshotMaxAge=2*time.Minute
 defaultOperationalStoreDriver="json"
 defaultTransactionStoreDriver="json"
	defaultAuditStoreDriver="memory"
)

type Config struct{StorePath,OperationalStoreDriver,TransactionStorePath,ProviderStateStorePath,TransactionStoreDriver,AuditStoreDriver,PostgresDSN,PostgresSchemaMode string;SyncInterval time.Duration;FailureThreshold int;Currency,CatalogStorePath,CatalogSyncStatusStorePath string;CatalogSyncInterval,CatalogMaxAge,OperationalSnapshotMaxAge time.Duration}
type databaseCloser interface { Close() error }

var ErrServiceClosed = errors.New("service is closed")
var ErrServiceLifecycleActive = errors.New("service lifecycle is still active")

var runtimeInitializationFailureHook func(string, *runtimeDatabaseOwnership) error

func runRuntimeInitializationFailureHook(stage string, ownership *runtimeDatabaseOwnership) error {
	if runtimeInitializationFailureHook == nil {
		return nil
	}
	return runtimeInitializationFailureHook(stage, ownership)
}

type runtimeDatabaseOwnership struct {
	mu sync.Mutex
	transactionDB databaseCloser
	auditDB databaseCloser
	operationalDB databaseCloser
	transferredToService bool
	closed bool
	closeErr error
}

func newRuntimeDatabaseOwnership(transactionDB, auditDB databaseCloser) *runtimeDatabaseOwnership {
	return &runtimeDatabaseOwnership{transactionDB: transactionDB, auditDB: auditDB}
}

func (o *runtimeDatabaseOwnership) transferToService() {
	if o == nil { return }
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed { return }
	o.transferredToService = true
}

func (o *runtimeDatabaseOwnership) transferred() bool {
	if o == nil { return false }
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.transferredToService
}

func (o *runtimeDatabaseOwnership) cleanupBeforeTransfer() error {
	if o == nil { return nil }
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.transferredToService || o.closed { return o.closeErr }
	o.closed = true
	o.closeErr = closeRuntimeDatabases(o.transactionDB, o.auditDB, o.operationalDB)
	return o.closeErr
}

func (o *runtimeDatabaseOwnership) isClosed() bool {
	if o == nil { return false }
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.closed
}

func (o *runtimeDatabaseOwnership) closeOwned() error {
	if o == nil { return nil }
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed { return o.closeErr }
	o.closed = true
	o.closeErr = closeRuntimeDatabases(o.transactionDB, o.auditDB, o.operationalDB)
	return o.closeErr
}

type catalogWorkerLifecycle struct {
	mu sync.Mutex
	running bool
	cancel context.CancelFunc
}

func newCatalogWorkerLifecycle() *catalogWorkerLifecycle { return &catalogWorkerLifecycle{} }

func (l *catalogWorkerLifecycle) Start(parent context.Context) (context.Context, error) {
	if parent == nil { return nil, errors.New("catalog parent context is required") }
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running { return nil, errors.New("catalog worker is already running") }
	ctx, cancel := context.WithCancel(parent)
	l.cancel = cancel
	l.running = true
	return ctx, nil
}

func (l *catalogWorkerLifecycle) Running() bool {
	if l == nil { return false }
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (l *catalogWorkerLifecycle) Shutdown() {
	if l == nil { return }
	l.mu.Lock()
	if !l.running { l.mu.Unlock(); return }
	cancel := l.cancel
	l.cancel = nil
	l.running = false
	l.mu.Unlock()
	cancel()
}

type Service struct{syncService *operational.SyncService;purchaseService *routing.Service;catalogSync *catalog.SyncService;providerState *operational.ProviderStateStore;databaseOwnership *runtimeDatabaseOwnership;balanceLifecycle *operational.SyncWorkerLifecycle;catalogLifecycle *catalogWorkerLifecycle;interval,catalogInterval time.Duration;catalogStart func(context.Context) (context.Context,error);balanceStart func(context.Context) error;balanceShutdown func(context.Context) error;catalogShutdown func() error;balanceShutdownCompleted bool;catalogShutdownCompleted bool;shutdownMu sync.Mutex}

// ProviderDiagnostics exposes provider-neutral administrative diagnostics to
// internal runtime callers. It never authorizes provider execution.
func (s *Service) ProviderDiagnostics() ([]operational.ProviderDiagnostic, error) {
	if s == nil || s.providerState == nil || s.purchaseService == nil || s.purchaseService.Router == nil {
		return nil, errors.New("provider runtime is not initialized")
	}
	admin, err := operational.NewProviderAdminService(s.providerState)
	if err != nil { return nil, err }
	return admin.DiagnoseAll(s.purchaseService.Router.Registry)
}

// ReconcileProviderCapabilities synchronizes persisted capability metadata
// after an explicit administrative drift investigation. It never enables a
// provider; callers must explicitly enable the lifecycle afterward.
func (s *Service) ReconcileProviderCapabilities(name string) (operational.ProviderDiagnostic, error) {
	if s == nil || s.providerState == nil || s.purchaseService == nil || s.purchaseService.Router == nil {
		return operational.ProviderDiagnostic{}, errors.New("provider runtime is not initialized")
	}
	admin, err := operational.NewProviderAdminService(s.providerState)
	if err != nil { return operational.ProviderDiagnostic{}, err }
	return admin.ReconcileCapabilityState(name, s.purchaseService.Router.Registry)
}

// EnableProvider explicitly changes only the operational lifecycle gate.
func (s *Service) EnableProvider(name string) (operational.ProviderState, error) {
	if s == nil || s.providerState == nil {
		return operational.ProviderState{}, errors.New("provider runtime is not initialized")
	}
	admin, err := operational.NewProviderAdminService(s.providerState)
	if err != nil { return operational.ProviderState{}, err }
	return admin.Enable(name)
}

func (s *Service) DisableProvider(name string) (operational.ProviderState, error) {
	if s == nil || s.providerState == nil {
		return operational.ProviderState{}, errors.New("provider runtime is not initialized")
	}
	admin, err := operational.NewProviderAdminService(s.providerState)
	if err != nil { return operational.ProviderState{}, err }
	return admin.Disable(name)
}

func (s *Service) EnableProviderCapability(name string, capability provider.Capability) (operational.ProviderState, error) {
	if s == nil || s.providerState == nil {
		return operational.ProviderState{}, errors.New("provider runtime is not initialized")
	}
	admin, err := operational.NewProviderAdminService(s.providerState)
	if err != nil { return operational.ProviderState{}, err }
	return admin.EnableCapability(name, operational.Capability(capability))
}

func (s *Service) DisableProviderCapability(name string, capability provider.Capability) (operational.ProviderState, error) {
	if s == nil || s.providerState == nil {
		return operational.ProviderState{}, errors.New("provider runtime is not initialized")
	}
	admin, err := operational.NewProviderAdminService(s.providerState)
	if err != nil { return operational.ProviderState{}, err }
	return admin.DisableCapability(name, operational.Capability(capability))
}

// ProviderRouteExplainabilitySnapshot returns a deterministic administrative snapshot
// across every registered provider and the canonical capability vocabulary. It is
// observational only and never authorizes provider execution.
func (s *Service) ProviderRouteExplainabilitySnapshot(ctx context.Context) (routing.AdministrativeRouteExplanationSnapshot, error) {
	if s == nil || s.purchaseService == nil || s.purchaseService.Router == nil {
		return routing.AdministrativeRouteExplanationSnapshot{}, errors.New("provider runtime is not initialized")
	}
	return routing.ExplainAllProviderRoutes(ctx, s.purchaseService.Router)
}

// ExplainProviderRoute returns deterministic internal routing/readiness diagnostics.
// It is observational only and never authorizes provider execution.
func (s *Service) ExplainProviderRoute(ctx context.Context, name string, capability provider.Capability, productCode string, amount int64) (routing.ProviderRouteExplanation, error) {
	if s == nil || s.purchaseService == nil || s.purchaseService.Router == nil { return routing.ProviderRouteExplanation{}, errors.New("provider runtime is not initialized") }
	return routing.ExplainProviderRoute(ctx, s.purchaseService.Router, name, capability, productCode, amount)
}


func (s *Service) CatalogSyncStatusPersistenceError() error {
	if s == nil || s.catalogSync == nil {
		return nil
	}
	return s.catalogSync.StatusPersistenceError()
}

func (s *Service) CatalogSyncStatusPersistenceFailures() int {
	if s == nil || s.catalogSync == nil {
		return 0
	}
	return s.catalogSync.StatusPersistenceFailures()
}

func (s *Service) CatalogSyncStatuses() []catalog.SyncStatus {
	if s == nil || s.catalogSync == nil {
		return nil
	}
	return s.catalogSync.Statuses()
}

func LoadConfig()(Config,error){
 cfg:=Config{StorePath:os.Getenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH"),OperationalStoreDriver:os.Getenv("DESKAPROVIDER_OPERATIONAL_STORE_DRIVER"),TransactionStoreDriver:os.Getenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER"),AuditStoreDriver:os.Getenv("DESKAPROVIDER_AUDIT_STORE_DRIVER"),PostgresDSN:os.Getenv("DESKAPROVIDER_POSTGRES_DSN"),PostgresSchemaMode:os.Getenv("DESKAPROVIDER_POSTGRES_SCHEMA_MODE"),ProviderStateStorePath:os.Getenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH"),TransactionStorePath:os.Getenv("DESKAPROVIDER_TRANSACTION_STORE_PATH"),SyncInterval:defaultSyncInterval,FailureThreshold:defaultFailureThreshold,Currency:os.Getenv("DESKAPROVIDER_OPERATIONAL_CURRENCY"),CatalogStorePath:os.Getenv("DESKAPROVIDER_CATALOG_STORE_PATH"),CatalogSyncStatusStorePath:os.Getenv("DESKAPROVIDER_CATALOG_SYNC_STATUS_STORE_PATH"),CatalogSyncInterval:defaultCatalogSyncInterval,CatalogMaxAge:defaultCatalogMaxAge,OperationalSnapshotMaxAge:defaultOperationalSnapshotMaxAge}
 if cfg.StorePath==""{cfg.StorePath=defaultStorePath};if _, ok := os.LookupEnv("DESKAPROVIDER_POSTGRES_SCHEMA_MODE"); !ok { cfg.PostgresSchemaMode="check" };if cfg.OperationalStoreDriver==""{cfg.OperationalStoreDriver=defaultOperationalStoreDriver};if cfg.OperationalStoreDriver!="json"&&cfg.OperationalStoreDriver!="postgres"{return Config{},fmt.Errorf("invalid DESKAPROVIDER_OPERATIONAL_STORE_DRIVER: %q",cfg.OperationalStoreDriver)};if cfg.CatalogSyncStatusStorePath==""{cfg.CatalogSyncStatusStorePath=defaultCatalogSyncStatusStorePath};if cfg.TransactionStoreDriver==""{cfg.TransactionStoreDriver=defaultTransactionStoreDriver};if cfg.AuditStoreDriver==""{cfg.AuditStoreDriver=defaultAuditStoreDriver};if cfg.AuditStoreDriver!="memory"&&cfg.AuditStoreDriver!="postgres"{return Config{},fmt.Errorf("invalid DESKAPROVIDER_AUDIT_STORE_DRIVER: %q",cfg.AuditStoreDriver)};if cfg.TransactionStoreDriver!="json"&&cfg.TransactionStoreDriver!="postgres"{return Config{},fmt.Errorf("invalid DESKAPROVIDER_TRANSACTION_STORE_DRIVER: %q",cfg.TransactionStoreDriver)};if cfg.PostgresSchemaMode!=""&&cfg.PostgresSchemaMode!="check"&&cfg.PostgresSchemaMode!="migrate"{return Config{},fmt.Errorf("invalid DESKAPROVIDER_POSTGRES_SCHEMA_MODE: %q",cfg.PostgresSchemaMode)};if cfg.PostgresDSN=="" {
  if cfg.OperationalStoreDriver=="postgres" { return Config{},errors.New("DESKAPROVIDER_POSTGRES_DSN is required when DESKAPROVIDER_OPERATIONAL_STORE_DRIVER=postgres") }
  if cfg.TransactionStoreDriver=="postgres" { return Config{},errors.New("DESKAPROVIDER_POSTGRES_DSN is required when DESKAPROVIDER_TRANSACTION_STORE_DRIVER=postgres") }
  if cfg.AuditStoreDriver=="postgres" { return Config{},errors.New("DESKAPROVIDER_POSTGRES_DSN is required when DESKAPROVIDER_AUDIT_STORE_DRIVER=postgres") }
 };if cfg.ProviderStateStorePath==""{cfg.ProviderStateStorePath=defaultProviderStateStorePath};if cfg.TransactionStorePath==""{cfg.TransactionStorePath=defaultTransactionStorePath};if cfg.Currency==""{cfg.Currency=defaultCurrency};if cfg.CatalogStorePath==""{cfg.CatalogStorePath=defaultCatalogStorePath}
 if raw:=os.Getenv("DESKAPROVIDER_CATALOG_SYNC_INTERVAL");raw!=""{v,e:=time.ParseDuration(raw);if e!=nil||v<=0{return Config{},fmt.Errorf("invalid DESKAPROVIDER_CATALOG_SYNC_INTERVAL: %q",raw)};cfg.CatalogSyncInterval=v}
 if raw:=os.Getenv("DESKAPROVIDER_CATALOG_MAX_AGE");raw!=""{v,e:=time.ParseDuration(raw);if e!=nil||v<=0{return Config{},fmt.Errorf("invalid DESKAPROVIDER_CATALOG_MAX_AGE: %q",raw)};cfg.CatalogMaxAge=v}
 if raw:=os.Getenv("DESKAPROVIDER_OPERATIONAL_SNAPSHOT_MAX_AGE");raw!=""{v,e:=time.ParseDuration(raw);if e!=nil||v<=0{return Config{},fmt.Errorf("invalid DESKAPROVIDER_OPERATIONAL_SNAPSHOT_MAX_AGE: %q",raw)};cfg.OperationalSnapshotMaxAge=v}
 if raw:=os.Getenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL");raw!=""{v,e:=time.ParseDuration(raw);if e!=nil||v<=0{return Config{},fmt.Errorf("invalid DESKAPROVIDER_BALANCE_SYNC_INTERVAL: %q",raw)};cfg.SyncInterval=v}
 if raw:=os.Getenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD");raw!=""{v,e:=strconv.Atoi(raw);if e!=nil||v<1{return Config{},fmt.Errorf("invalid DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD: %q",raw)};cfg.FailureThreshold=v}
 return cfg,nil
}

func NewFromEnvironment(httpClient *http.Client)(*Service,error){
 return NewFromEnvironmentContext(context.Background(),httpClient)
}

func NewFromEnvironmentContext(ctx context.Context,httpClient *http.Client)(service *Service, err error){
 if ctx==nil{return nil,errors.New("initialization context is required")}
 if err:=ctx.Err();err!=nil{return nil,err}
 cfg,e:=LoadConfig();if e!=nil{return nil,e}
 var cached provider.PPOBProvider
 digiConfiguredUser, digiUserSet := os.LookupEnv("DIGIFLAZZ_USERNAME")
 digiConfiguredKey, digiKeySet := os.LookupEnv("DIGIFLAZZ_API_KEY")
 if digiUserSet || digiKeySet || strings.TrimSpace(digiConfiguredUser) != "" || strings.TrimSpace(digiConfiguredKey) != "" {
  digiCfg,e:=config.LoadDigiFlazzConfig();if e!=nil{return nil,e}
  client,e:=digiflazz.New(digiCfg,httpClient);if e!=nil{return nil,e}
  cachedClient,e:=digiflazz.NewCachedClient(client,defaultPriceListCacheTTL);if e!=nil{return nil,e}
  cached=cachedClient
 }
 registry:=provider.NewRegistry()
 if e=registerConfiguredProviders(registry,cached,httpClient);e!=nil{return nil,e}
 var store operational.Store
 var operationalDB *sql.DB
 catalogStore,e:=catalog.NewJSONFileStore(cfg.CatalogStorePath);if e!=nil{return nil,e}
 statusPersistence,e:=catalog.NewJSONFileStatusPersistence(cfg.CatalogSyncStatusStorePath);if e!=nil{return nil,e}
 catalogSync,e:=catalog.NewSyncServiceWithStatusPersistence(registry,catalogStore,statusPersistence);if e!=nil{return nil,e}
 transactionStore,transactionDB,e:=openTransactionStore(ctx,cfg);if e!=nil{return nil,e}
auditStore,auditDB,e:=openAuditStore(ctx,cfg,transactionDB);if e!=nil{return nil,withRuntimeInitializationCleanupError(e,transactionDB,auditDB)}
ownership:=newRuntimeDatabaseOwnership(transactionDB,auditDB)
defer func(){
	if ownership == nil || ownership.transferred() { return }
	if cleanupErr:=ownership.cleanupBeforeTransfer();cleanupErr!=nil {
		err=combineRuntimeShutdownError(err,cleanupErr)
		service=nil
	}
}()
if e:=runRuntimeInitializationFailureHook("after-database-acquisition", ownership); e!=nil { return nil,e }
store, operationalDB, e = openOperationalStore(ctx, cfg, transactionDB)
if e != nil { return nil, e }
ownership.operationalDB = operationalDB
syncService, e := operational.NewSyncService(registry, store, cfg.Currency, cfg.FailureThreshold)
if e != nil { return nil, e }
statePersistence,e:=operational.NewJSONFileProviderStateStore(cfg.ProviderStateStorePath);if e!=nil{return nil,e}
stateStore,e:=operational.NewPersistentProviderStateStore(statePersistence);if e!=nil{return nil,e}
if e:=runRuntimeInitializationFailureHook("after-provider-state-store", ownership); e!=nil { return nil,e }
for _, name := range registry.Names() {
		state, ok := stateStore.Get(name)
		if !ok {
			state, e = operational.NewProviderState(name)
			if e != nil { return nil, e }
		}
		descriptor, e := registry.Capabilities(name)
		if e != nil { return nil, e }
		drift := operational.DetectCapabilityDrift(state, descriptor)
		if !ok {
			// Seed a missing provider with the explicit disabled lifecycle. The
			// subsequent reconciliation remains the single mutation boundary for
			// capability metadata and preserves that lifecycle gate.
			if e = stateStore.Put(state); e != nil { return nil, e }
		}
		if _, e = stateStore.ReconcileCapabilityState(
			name,
			capabilitiesFromDescriptor(descriptor),
			operational.CapabilityMetadataFingerprint(descriptor),
			drift.Drifted(),
		); e != nil {
			return nil, e
		}
	} // persist synchronized provider lifecycle/capability state before router construction
router,e:=routing.NewWithCatalogAndStateAndOperationalMaxAge(registry,store,nil,catalogStore,stateStore,cfg.OperationalSnapshotMaxAge);if e!=nil{return nil,e}
if e:=runRuntimeInitializationFailureHook("after-router", ownership); e!=nil { return nil,e }
purchaseService,e:=routing.NewServiceWithStoreContextAndAudit(ctx,router,transactionStore,auditStore);if e!=nil{return nil,e}
if e:=runRuntimeInitializationFailureHook("after-purchase-service", ownership); e!=nil { return nil,e }
 balanceLifecycle,e:=operational.NewSyncWorkerLifecycle(syncService,cfg.SyncInterval);if e!=nil{return nil,e}
service=&Service{syncService:syncService,purchaseService:purchaseService,catalogSync:catalogSync,providerState:stateStore,databaseOwnership:ownership,balanceLifecycle:balanceLifecycle,catalogLifecycle:newCatalogWorkerLifecycle(),interval:cfg.SyncInterval,catalogInterval:cfg.CatalogSyncInterval}
if e:=runRuntimeInitializationFailureHook("before-ownership-transfer", ownership); e!=nil { return nil,e }
if err := checkRuntimeInitializationContext(ctx); err != nil { return nil, err }
ownership.transferToService()
return service,nil
}

func capabilitiesFromDescriptor(descriptor provider.CapabilityDescriptor) []operational.Capability {
	capabilities := make([]operational.Capability, 0, len(descriptor.Capabilities))
	for capability, status := range descriptor.Capabilities {
		if status.AdapterImplemented {
			capabilities = append(capabilities, operational.Capability(capability))
		}
	}
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
	return capabilities
}

func registerConfiguredProviders(registry *provider.Registry, digi provider.PPOBProvider, httpClient *http.Client) error {
 if registry == nil { return errors.New("provider registry is required") }

 // Capability metadata is explicit at registration time. A capability is
 // listed only when the concrete adapter contract is implemented and covered
 // by deterministic repository tests; Enabled and LiveTested remain false.
 digiStatus := provider.CapabilityStatus{Verified:true, Configured:true, AdapterImplemented:true, Tested:true, Enabled:false, LiveTested:false}
 digiCapabilities := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
  provider.CapabilityPPOB: digiStatus,
  provider.CapabilityBalance: digiStatus,
  provider.CapabilityWebhook: digiStatus,
  provider.CapabilityCatalog: digiStatus,
 }}
 if digi != nil {
  if err := registry.RegisterWithCapabilities("digiflazz", digi, digiCapabilities); err != nil { return err }
 }

 if strings.TrimSpace(os.Getenv("MIDTRANS_SERVER_KEY")) != "" {
  midCfg, err := config.LoadMidtransConfig()
  if err != nil { return err }
  midClient, err := midtrans.New(midCfg, httpClient)
  if err != nil { return err }
  midStatus := provider.CapabilityStatus{Verified:true, Configured:true, AdapterImplemented:true, Tested:true, Enabled:false, LiveTested:false}
  if err := registry.RegisterCapabilityProvider("midtrans", provider.CapabilityPayment, midClient, midStatus); err != nil { return err }
  if err := registry.RegisterCapabilityProvider("midtrans", provider.CapabilityWebhook, midClient, midStatus); err != nil { return err }
 }

 if os.Getenv("IAK_USERNAME") == "" && os.Getenv("IAK_API_KEY") == "" {
  return registerConfiguredXPSindonesia(registry, httpClient)
 }
 iakCfg, err := config.LoadIAKConfig(); if err != nil { return err }
 iakClient, err := iak.New(iakCfg, httpClient); if err != nil { return err }
 iakStatus := provider.CapabilityStatus{Verified:false, Configured:true, AdapterImplemented:true, Tested:true, Enabled:false, LiveTested:false}
 iakCapabilities := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
  provider.CapabilityPPOB: iakStatus,
  provider.CapabilityBalance: iakStatus,
  provider.CapabilityWebhook: iakStatus,
  provider.CapabilityCatalog: iakStatus,
 }}
 if err := registry.RegisterWithCapabilities("iak", iakClient, iakCapabilities); err != nil { return err }
 return registerConfiguredXPSindonesia(registry, httpClient)
}

func registerConfiguredXPSindonesia(registry *provider.Registry, httpClient *http.Client) error {
 configured := strings.TrimSpace(os.Getenv("XP_SINDONESIA_ID")) != "" ||
  strings.TrimSpace(os.Getenv("XP_SINDONESIA_KEY")) != "" ||
  strings.TrimSpace(os.Getenv("XP_SINDONESIA_API")) != ""
 if !configured { return nil }

 cfg, err := config.LoadXPSindonesiaConfig()
 if err != nil { return err }
 client, err := xpsindonesia.New(cfg, httpClient)
 if err != nil { return err }
 status := provider.CapabilityStatus{Verified:false, Configured:true, AdapterImplemented:true, Tested:true, Enabled:false, LiveTested:false}
 capabilities := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
  provider.CapabilityPPOB: status,
  provider.CapabilityBalance: status,
  provider.CapabilityWebhook: status,
 }}
 return registry.RegisterWithCapabilities("xp-sindonesia", client, capabilities)
}

func New(syncService *operational.SyncService,interval time.Duration)(*Service,error){if syncService==nil{return nil,errors.New("sync service is required")};if interval<=0{return nil,errors.New("sync interval must be greater than zero")};balanceLifecycle,err:=operational.NewSyncWorkerLifecycle(syncService,interval);if err!=nil{return nil,err};return &Service{syncService:syncService,balanceLifecycle:balanceLifecycle,catalogLifecycle:newCatalogWorkerLifecycle(),interval:interval},nil}
func runtimeShutdownContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithCancel(context.Background())
	}
	if deadline, ok := parent.Deadline(); ok {
		return context.WithDeadline(context.Background(), deadline)
	}
	return context.WithCancel(context.Background())
}

func (s *Service) Run(ctx context.Context) error {
	if ctx == nil { return errors.New("context is required") }
	s.shutdownMu.Lock()
	if s.databaseOwnership != nil && s.databaseOwnership.isClosed() {
		s.shutdownMu.Unlock()
		return ErrServiceClosed
	}
	if s.balanceLifecycle != nil && s.balanceLifecycle.Running() {
		s.shutdownMu.Unlock()
		return operational.ErrSyncWorkerRunning
	}
	if s.catalogLifecycle != nil && s.catalogLifecycle.Running() {
		s.shutdownMu.Unlock()
		return ErrServiceLifecycleActive
	}

catalogStarted := false
	shutdownLocked := func(primary, workerErr error) error {
		var catalogErr error
		if catalogStarted {
			catalogErr = s.shutdownCatalogLifecycle()
		}
		var closeErr error
		balanceStopped := s.balanceLifecycle == nil || !s.balanceLifecycle.Running()
		catalogStopped := !catalogStarted || s.catalogLifecycle == nil || !s.catalogLifecycle.Running()
		if balanceStopped && catalogStopped {
			closeErr = s.closeOwnedDatabases()
		}
		return combineRuntimeShutdownError(combineRuntimeShutdownError(combineRuntimeShutdownError(primary, workerErr), catalogErr), closeErr)
	}
	if s.balanceLifecycle == nil {
		if s.catalogSync == nil {
			runErr := s.syncService.Run(ctx, s.interval)
			return combineRuntimeShutdownError(runErr, s.Close())
		}
		_ = s.catalogSync.SyncAll(ctx)
		ticker := time.NewTicker(s.catalogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return combineRuntimeShutdownError(ctx.Err(), s.Close())
			case <-ticker.C:
				_ = s.catalogSync.SyncAll(ctx)
			}
		}
	}

	startBalance := s.balanceLifecycle.Start
	if s.balanceStart != nil {
		startBalance = s.balanceStart
	}
	s.balanceShutdownCompleted = false
	workerShutdownCtx, cancel := runtimeShutdownContext(ctx)
	if err := startBalance(ctx); err != nil {
		if !errors.Is(err, operational.ErrSyncWorkerRunning) {
			workerErr := s.rollbackStartedLifecycles(workerShutdownCtx)
			s.shutdownMu.Unlock()
			cancel()
			return combineRuntimeShutdownError(combineRuntimeShutdownError(err, workerErr), s.Close())
		}
		s.shutdownMu.Unlock()
		cancel()
		return err
	}
	s.shutdownMu.Unlock()
	defer cancel()

	workerDone := s.balanceLifecycle.Done()
	if s.catalogSync == nil {
		select {
		case <-ctx.Done():
			s.shutdownMu.Lock()
			defer s.shutdownMu.Unlock()
			workerErr := s.shutdownBalanceWorker(workerShutdownCtx)
			return shutdownLocked(ctx.Err(), workerErr)
		case <-workerDone:
			s.shutdownMu.Lock()
			defer s.shutdownMu.Unlock()
			workerErr := s.balanceLifecycle.ExitError()
			if workerErr == nil {
				workerErr = operational.ErrSyncWorkerExited
			} else if errors.Is(workerErr, context.Canceled) && ctx.Err() == nil {
				workerErr = operational.ErrSyncWorkerExited
			}
			return shutdownLocked(workerErr, nil)
		}
	}

	startCatalog := s.catalogLifecycle.Start
	if s.catalogStart != nil {
		startCatalog = s.catalogStart
	}
	catalogCtx, catalogStartErr := startCatalog(ctx)
	if catalogStartErr != nil {
		s.shutdownMu.Lock()
		defer s.shutdownMu.Unlock()
		workerErr := s.rollbackStartedLifecycles(workerShutdownCtx)
		return shutdownLocked(catalogStartErr, workerErr)
	}
	catalogStarted = true
	s.catalogShutdownCompleted = false
	_ = s.catalogSync.SyncAll(catalogCtx)
	ticker := time.NewTicker(s.catalogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.shutdownMu.Lock()
			defer s.shutdownMu.Unlock()
			workerErr := s.shutdownBalanceWorker(workerShutdownCtx)
			return shutdownLocked(ctx.Err(), workerErr)
		case <-workerDone:
			s.shutdownMu.Lock()
			defer s.shutdownMu.Unlock()
			workerErr := s.balanceLifecycle.ExitError()
			if workerErr == nil {
				workerErr = operational.ErrSyncWorkerExited
			} else if errors.Is(workerErr, context.Canceled) && ctx.Err() == nil {
				workerErr = operational.ErrSyncWorkerExited
			}
			return shutdownLocked(workerErr, nil)
		case <-ticker.C:
			_ = s.catalogSync.SyncAll(catalogCtx)
		}
	}
}

func (s *Service) PurchaseService()*routing.Service{if s==nil{return nil};return s.purchaseService}
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.shutdownMu.Lock()
	defer s.shutdownMu.Unlock()
	if s.balanceLifecycle != nil && s.balanceLifecycle.Running() {
		return errors.New("service close requires worker shutdown")
	}
	if s.catalogLifecycle != nil && s.catalogLifecycle.Running() {
		return errors.New("service close requires catalog worker shutdown")
	}
	return s.closeOwnedDatabases()
}

func (s *Service) replaceDatabaseOwnership(next *runtimeDatabaseOwnership) error {
	if s == nil {
		return nil
	}
	s.shutdownMu.Lock()
	defer s.shutdownMu.Unlock()

	if (s.balanceLifecycle != nil && s.balanceLifecycle.Running()) ||
		(s.catalogLifecycle != nil && s.catalogLifecycle.Running()) {
		return ErrServiceLifecycleActive
	}

	current := s.databaseOwnership
	if current != nil && current != next && !current.isClosed() {
		if err := current.closeOwned(); err != nil {
			return err
		}
	}
	s.databaseOwnership = next
	return nil
}

func (s *Service) closeOwnedDatabases() error { if s==nil || s.databaseOwnership==nil { return nil }; return s.databaseOwnership.closeOwned() }

func (s *Service) shutdownBalanceWorker(ctx context.Context) error {
	if s == nil || s.balanceLifecycle == nil || s.balanceShutdownCompleted { return nil }
	var err error
	if s.balanceShutdown != nil {
		err = s.balanceShutdown(ctx)
	} else {
		err = s.balanceLifecycle.Shutdown(ctx)
	}
	if !s.balanceLifecycle.Running() {
		s.balanceShutdownCompleted = true
	}
	return err
}

func (s *Service) rollbackStartedLifecycles(ctx context.Context) error {
	if s == nil { return nil }
	var err error
	if s.balanceLifecycle != nil && s.balanceLifecycle.Running() {
		err = s.shutdownBalanceWorker(ctx)
	}
	return err
}

func (s *Service) shutdownCatalogLifecycle() error {
	if s == nil || s.catalogLifecycle == nil || s.catalogShutdownCompleted { return nil }
	var err error
	if s.catalogShutdown != nil {
		err = s.catalogShutdown()
	} else {
		s.catalogLifecycle.Shutdown()
	}
	if !s.catalogLifecycle.Running() {
		s.catalogShutdownCompleted = true
	}
	return err
}

func checkRuntimeInitializationContext(ctx context.Context) error {
	if ctx == nil { return errors.New("initialization context is required") }
	return ctx.Err()
}

func combineRuntimeShutdownError(primary,closeErr error) error{if primary==nil{return closeErr};if closeErr==nil{return primary};return errors.Join(primary,closeErr)}

func withRuntimeInitializationCleanupError(primary error,transactionDB,auditDB databaseCloser) error{if primary==nil{return closeRuntimeDatabases(transactionDB,auditDB,nil)};return combineRuntimeShutdownError(primary,closeRuntimeDatabases(transactionDB,auditDB,nil))}

func closeRuntimeDatabases(transactionDB, auditDB databaseCloser, operationalDBs ...databaseCloser) error{
	var operationalDB databaseCloser
	if len(operationalDBs) > 0 { operationalDB = operationalDBs[0] }
	var errs []error
	if databaseCloserIsNil(transactionDB)==false {
		if err:=transactionDB.Close();err!=nil{errs=append(errs,fmt.Errorf("close transaction database: %w",err))}
	}
	if databaseCloserIsNil(auditDB)==false && auditDB!=transactionDB && auditDB!=operationalDB {
		if err:=auditDB.Close();err!=nil{errs=append(errs,fmt.Errorf("close audit database: %w",err))}
	}
	if databaseCloserIsNil(operationalDB)==false && operationalDB!=transactionDB && operationalDB!=auditDB {
		if err:=operationalDB.Close();err!=nil{errs=append(errs,fmt.Errorf("close operational database: %w",err))}
	}
	return errors.Join(errs...)
}

func databaseCloserIsNil(value databaseCloser) bool {
	if value == nil { return true }
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func openOperationalStore(ctx context.Context, cfg Config, transactionDB *sql.DB) (operational.Store, *sql.DB, error) {
	if err := ctx.Err(); err != nil { return nil, nil, err }
	if cfg.OperationalStoreDriver != "postgres" {
		store, err := operational.NewJSONFileStore(cfg.StorePath)
		if err != nil { return nil, nil, err }
		return store, nil, nil
	}
	db := transactionDB
	owned := false
	if db == nil {
		var err error
		db, err = sql.Open("pgx", cfg.PostgresDSN)
		if err != nil { return nil, nil, fmt.Errorf("open PostgreSQL operational store: %w", err) }
		owned = true
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("ping PostgreSQL operational store: %w", err)
		}
	}
	if cfg.PostgresSchemaMode != "" {
		if err := preparePostgresSchema(ctx, db, cfg.PostgresSchemaMode, 2); err != nil {
			if owned { _ = db.Close() }
			return nil, nil, err
		}
	}
	store, err := operational.NewPostgresStore(db)
	if err != nil {
		if owned { _ = db.Close() }
		return nil, nil, err
	}
	if owned { return store, db, nil }
	return store, nil, nil
}

func preparePostgresSchema(ctx context.Context, db *sql.DB, mode string, version int) error {
	if db == nil { return errors.New("PostgreSQL schema database is required") }
	versions := []int{version}
	if version == 3 {
		versions = []int{1, 3}
	}
	if err := migrations.ValidateVersionSet(versions...); err != nil { return err }
	if mode == "migrate" {
		if err := migrations.Apply(ctx, db, versions...); err != nil { return fmt.Errorf("apply PostgreSQL provider migration %d: %w", version, err) }
		return nil
	}
	if version == 3 {
		if err := checkPostgresMigrationReadiness(ctx, db, 1); err != nil { return err }
	}
	return checkPostgresMigrationReadiness(ctx, db, version)
}

func checkPostgresMigrationReadiness(ctx context.Context, db *sql.DB, version int) error {
	if version == 1 {
		var transactions, audit bool
		if err := db.QueryRowContext(ctx, "SELECT to_regclass(current_schema() || '.provider_transactions') IS NOT NULL").Scan(&transactions); err != nil { return fmt.Errorf("check PostgreSQL migration 1 transaction schema: %w", err) }
		if err := db.QueryRowContext(ctx, "SELECT to_regclass(current_schema() || '.provider_transaction_audit') IS NOT NULL").Scan(&audit); err != nil { return fmt.Errorf("check PostgreSQL migration 1 audit schema: %w", err) }
		if !transactions || !audit { return errors.New("PostgreSQL migration 1 is not ready: apply DesKaProvider/backend/migrations/001_provider_transactions.sql") }
		return nil
	}
	if version == 2 { return checkOperationalSchema(ctx, db) }
	if version == 3 { return checkPaymentTransactionSchema(ctx, db) }
	return fmt.Errorf("unsupported PostgreSQL migration readiness check: %d", version)
}

func checkPaymentTransactionSchema(ctx context.Context, db *sql.DB) error {
	if db == nil { return errors.New("payment transaction PostgreSQL database is required") }
	var columns int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'provider_transactions' AND column_name IN ('transaction_kind','payment_provider_reference','payment_currency','payment_customer_id','payment_description')`).Scan(&columns)
	if err != nil { return fmt.Errorf("check payment transaction schema: %w", err) }
	if columns != 5 { return errors.New("payment transaction schema is not ready: apply DesKaProvider/backend/migrations/003_payment_transactions.sql") }
	return nil
}

func checkOperationalSchema(ctx context.Context, db *sql.DB) error {
	if db == nil { return errors.New("operational PostgreSQL database is required") }
	var columns int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'provider_operational_snapshots' AND column_name IN ('provider_name','balance','currency','health','last_checked_at','last_success_at','last_error','consecutive_failures')`).Scan(&columns)
	if err != nil { return fmt.Errorf("check provider operational schema: %w", err) }
	if columns != 8 { return errors.New("provider operational schema is not ready: apply DesKaProvider/backend/migrations/002_provider_operational_snapshots.sql") }
	return nil
}

func openAuditStore(ctx context.Context, cfg Config, transactionDB *sql.DB) (routing.TransactionAuditStore, *sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if cfg.AuditStoreDriver != "postgres" {
		store := routing.NewMemoryTransactionAuditStore()
		return store, nil, nil
	}
	if transactionDB != nil {
		if cfg.PostgresSchemaMode != "" { if err := preparePostgresSchema(ctx, transactionDB, cfg.PostgresSchemaMode, 1); err != nil { return nil, nil, err } }
		store, err := routing.NewPostgresTransactionAuditStore(transactionDB)
		if err != nil {
			return nil, nil, err
		}
		return store, nil, nil
	}
	db, err := sql.Open("pgx", cfg.PostgresDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("open PostgreSQL audit store: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping PostgreSQL audit store: %w", err)
	}
	if cfg.PostgresSchemaMode != "" { if err := preparePostgresSchema(ctx, db, cfg.PostgresSchemaMode, 3); err != nil { _ = db.Close(); return nil, nil, err } }
	store, err := routing.NewPostgresTransactionAuditStore(db)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return store, db, nil
}

func openTransactionStore(ctx context.Context, cfg Config) (routing.TransactionStore, *sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if cfg.TransactionStoreDriver == "postgres" {
		db, err := sql.Open("pgx", cfg.PostgresDSN)
		if err != nil {
			return nil, nil, fmt.Errorf("open PostgreSQL transaction store: %w", err)
		}
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("ping PostgreSQL transaction store: %w", err)
		}
		if cfg.PostgresSchemaMode != "" { if err := preparePostgresSchema(ctx, db, cfg.PostgresSchemaMode, 3); err != nil { _ = db.Close(); return nil, nil, err } }
		store, err := routing.NewPostgresTransactionStore(db)
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		return store, db, nil
	}
	store, err := routing.NewJSONFileTransactionStore(cfg.TransactionStorePath)
	if err != nil {
		return nil, nil, err
	}
	return store, nil, nil
}
