package runtime

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
 "net/http"
 "os"
 "strconv"
 "time"

 _ "github.com/jackc/pgx/v5/stdlib"

 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
 digiflazz "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/DigiFlazz"
 iak "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/IAK"
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
 defaultCatalogSyncInterval=15*time.Minute
 defaultCatalogMaxAge=30*time.Minute
 defaultOperationalSnapshotMaxAge=2*time.Minute
 defaultTransactionStoreDriver="json"
	defaultAuditStoreDriver="memory"
)

type Config struct{StorePath,TransactionStorePath,ProviderStateStorePath,TransactionStoreDriver,AuditStoreDriver,PostgresDSN string;SyncInterval time.Duration;FailureThreshold int;Currency,CatalogStorePath string;CatalogSyncInterval,CatalogMaxAge,OperationalSnapshotMaxAge time.Duration}
type Service struct{syncService *operational.SyncService;purchaseService *routing.Service;catalogSync *catalog.SyncService;providerState *operational.ProviderStateStore;transactionDB *sql.DB;auditDB *sql.DB;interval,catalogInterval time.Duration}

func LoadConfig()(Config,error){
 cfg:=Config{StorePath:os.Getenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH"),TransactionStoreDriver:os.Getenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER"),AuditStoreDriver:os.Getenv("DESKAPROVIDER_AUDIT_STORE_DRIVER"),PostgresDSN:os.Getenv("DESKAPROVIDER_POSTGRES_DSN"),ProviderStateStorePath:os.Getenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH"),TransactionStorePath:os.Getenv("DESKAPROVIDER_TRANSACTION_STORE_PATH"),SyncInterval:defaultSyncInterval,FailureThreshold:defaultFailureThreshold,Currency:os.Getenv("DESKAPROVIDER_OPERATIONAL_CURRENCY"),CatalogStorePath:os.Getenv("DESKAPROVIDER_CATALOG_STORE_PATH"),CatalogSyncInterval:defaultCatalogSyncInterval,CatalogMaxAge:defaultCatalogMaxAge,OperationalSnapshotMaxAge:defaultOperationalSnapshotMaxAge}
 if cfg.StorePath==""{cfg.StorePath=defaultStorePath};if cfg.TransactionStoreDriver==""{cfg.TransactionStoreDriver=defaultTransactionStoreDriver};if cfg.AuditStoreDriver==""{cfg.AuditStoreDriver=defaultAuditStoreDriver};if cfg.AuditStoreDriver!="memory"&&cfg.AuditStoreDriver!="postgres"{return Config{},fmt.Errorf("invalid DESKAPROVIDER_AUDIT_STORE_DRIVER: %q",cfg.AuditStoreDriver)};if cfg.TransactionStoreDriver!="json"&&cfg.TransactionStoreDriver!="postgres"{return Config{},fmt.Errorf("invalid DESKAPROVIDER_TRANSACTION_STORE_DRIVER: %q",cfg.TransactionStoreDriver)};if (cfg.TransactionStoreDriver=="postgres"||cfg.AuditStoreDriver=="postgres")&&cfg.PostgresDSN==""{return Config{},errors.New("DESKAPROVIDER_POSTGRES_DSN is required when DESKAPROVIDER_TRANSACTION_STORE_DRIVER=postgres")};if cfg.ProviderStateStorePath==""{cfg.ProviderStateStorePath=defaultProviderStateStorePath};if cfg.TransactionStorePath==""{cfg.TransactionStorePath=defaultTransactionStorePath};if cfg.Currency==""{cfg.Currency=defaultCurrency};if cfg.CatalogStorePath==""{cfg.CatalogStorePath=defaultCatalogStorePath}
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

func NewFromEnvironmentContext(ctx context.Context,httpClient *http.Client)(*Service,error){
 if ctx==nil{return nil,errors.New("initialization context is required")}
 if err:=ctx.Err();err!=nil{return nil,err}
 cfg,e:=LoadConfig();if e!=nil{return nil,e}
 digiCfg,e:=config.LoadDigiFlazzConfig();if e!=nil{return nil,e}
 client,e:=digiflazz.New(digiCfg,httpClient);if e!=nil{return nil,e}
 cached,e:=digiflazz.NewCachedClient(client,defaultPriceListCacheTTL);if e!=nil{return nil,e}
 registry:=provider.NewRegistry();if e=registry.Register("digiflazz",cached);e!=nil{return nil,e}
 if os.Getenv("IAK_USERNAME")!=""||os.Getenv("IAK_API_KEY")!="" {
  iakCfg,e:=config.LoadIAKConfig();if e!=nil{return nil,e}
  iakClient,e:=iak.New(iakCfg,httpClient);if e!=nil{return nil,e}
  if e=registry.Register("iak",iakClient);e!=nil{return nil,e}
 }
 store,e:=operational.NewJSONFileStore(cfg.StorePath);if e!=nil{return nil,e}
 syncService,e:=operational.NewSyncService(registry,store,cfg.Currency,cfg.FailureThreshold);if e!=nil{return nil,e}
 catalogStore,e:=catalog.NewJSONFileStore(cfg.CatalogStorePath);if e!=nil{return nil,e}
 catalogSync,e:=catalog.NewSyncService(registry,catalogStore);if e!=nil{return nil,e}
 transactionStore,transactionDB,e:=openTransactionStore(ctx,cfg);if e!=nil{return nil,e}
 auditStore,auditDB,e:=openAuditStore(ctx,cfg,transactionDB);if e!=nil{if transactionDB!=nil{_ = transactionDB.Close()};return nil,e}
 statePersistence,e:=operational.NewJSONFileProviderStateStore(cfg.ProviderStateStorePath);if e!=nil{return nil,e}
 stateStore,e:=operational.NewPersistentProviderStateStore(statePersistence);if e!=nil{return nil,e}
 for _, name:=range registry.Names(){state,ok:=stateStore.Get(name);if !ok{state,e=operational.NewProviderState(name);if e!=nil{return nil,e}};state.Capabilities=[]operational.Capability{operational.CapabilityPPOB,operational.CapabilityBalance,operational.CapabilityWebhook};if e=stateStore.Put(state);e!=nil{return nil,e}}
 router,e:=routing.NewWithCatalogAndStateAndOperationalMaxAge(registry,store,nil,catalogStore,stateStore,cfg.OperationalSnapshotMaxAge);if e!=nil{return nil,e}
 purchaseService,e:=routing.NewServiceWithStoreContextAndAudit(ctx,router,transactionStore,auditStore);if e!=nil{return nil,e}
 return &Service{syncService:syncService,purchaseService:purchaseService,catalogSync:catalogSync,providerState:stateStore,transactionDB:transactionDB,auditDB:auditDB,interval:cfg.SyncInterval,catalogInterval:cfg.CatalogSyncInterval},nil
}

func New(syncService *operational.SyncService,interval time.Duration)(*Service,error){if syncService==nil{return nil,errors.New("sync service is required")};if interval<=0{return nil,errors.New("sync interval must be greater than zero")};return &Service{syncService:syncService,interval:interval},nil}
func (s *Service) Run(ctx context.Context)error{if ctx==nil{return errors.New("context is required")};if s.catalogSync==nil{err:=s.syncService.Run(ctx,s.interval);if s.transactionDB!=nil{_ = s.transactionDB.Close()};if s.auditDB!=nil{_ = s.auditDB.Close()};return err};_=s.catalogSync.SyncAll(ctx);ticker:=time.NewTicker(s.catalogInterval);defer ticker.Stop();go func(){_=s.syncService.Run(ctx,s.interval)}();for{select{case<-ctx.Done():if s.transactionDB!=nil{_ = s.transactionDB.Close()};if s.auditDB!=nil{_ = s.auditDB.Close()};return ctx.Err();case<-ticker.C:_=s.catalogSync.SyncAll(ctx)}}}
func (s *Service) PurchaseService()*routing.Service{if s==nil{return nil};return s.purchaseService}


func openAuditStore(ctx context.Context, cfg Config, transactionDB *sql.DB) (routing.TransactionAuditStore, *sql.DB, error) {
	if err := ctx.Err(); err != nil { return nil, nil, err }
	if cfg.AuditStoreDriver != "postgres" { store, err := routing.NewMemoryTransactionAuditStore(); return store, nil, err }
	if transactionDB != nil { return routing.NewPostgresTransactionAuditStore(transactionDB) }
	db, err := sql.Open("pgx", cfg.PostgresDSN)
	if err != nil { return nil, nil, fmt.Errorf("open PostgreSQL audit store: %w", err) }
	if err := db.PingContext(ctx); err != nil { _ = db.Close(); return nil, nil, fmt.Errorf("ping PostgreSQL audit store: %w", err) }
	store, err := routing.NewPostgresTransactionAuditStore(db)
	if err != nil { _ = db.Close(); return nil, nil, err }
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
