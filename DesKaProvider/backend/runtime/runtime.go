package runtime

import (
 "context"
 "errors"
 "fmt"
 "net/http"
 "os"
 "strconv"
 "time"

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
 defaultTransactionStorePath="data/provider-transactions.json"
 defaultSyncInterval=30*time.Second
 defaultFailureThreshold=3
 defaultCurrency="IDR"
 defaultPriceListCacheTTL=15*time.Minute
 defaultCatalogStorePath="data/product-catalog.json"
 defaultCatalogSyncInterval=15*time.Minute
 defaultCatalogMaxAge=30*time.Minute
)

type Config struct{StorePath,TransactionStorePath string;SyncInterval time.Duration;FailureThreshold int;Currency,CatalogStorePath string;CatalogSyncInterval,CatalogMaxAge time.Duration}
type Service struct{syncService *operational.SyncService;purchaseService *routing.Service;catalogSync *catalog.SyncService;providerState *operational.ProviderStateStore;interval,catalogInterval time.Duration}

func LoadConfig()(Config,error){
 cfg:=Config{StorePath:os.Getenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH"),TransactionStorePath:os.Getenv("DESKAPROVIDER_TRANSACTION_STORE_PATH"),SyncInterval:defaultSyncInterval,FailureThreshold:defaultFailureThreshold,Currency:os.Getenv("DESKAPROVIDER_OPERATIONAL_CURRENCY"),CatalogStorePath:os.Getenv("DESKAPROVIDER_CATALOG_STORE_PATH"),CatalogSyncInterval:defaultCatalogSyncInterval,CatalogMaxAge:defaultCatalogMaxAge}
 if cfg.StorePath==""{cfg.StorePath=defaultStorePath};if cfg.TransactionStorePath==""{cfg.TransactionStorePath=defaultTransactionStorePath};if cfg.Currency==""{cfg.Currency=defaultCurrency};if cfg.CatalogStorePath==""{cfg.CatalogStorePath=defaultCatalogStorePath}
 if raw:=os.Getenv("DESKAPROVIDER_CATALOG_SYNC_INTERVAL");raw!=""{v,e:=time.ParseDuration(raw);if e!=nil||v<=0{return Config{},fmt.Errorf("invalid DESKAPROVIDER_CATALOG_SYNC_INTERVAL: %q",raw)};cfg.CatalogSyncInterval=v}
 if raw:=os.Getenv("DESKAPROVIDER_CATALOG_MAX_AGE");raw!=""{v,e:=time.ParseDuration(raw);if e!=nil||v<=0{return Config{},fmt.Errorf("invalid DESKAPROVIDER_CATALOG_MAX_AGE: %q",raw)};cfg.CatalogMaxAge=v}
 if raw:=os.Getenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL");raw!=""{v,e:=time.ParseDuration(raw);if e!=nil||v<=0{return Config{},fmt.Errorf("invalid DESKAPROVIDER_BALANCE_SYNC_INTERVAL: %q",raw)};cfg.SyncInterval=v}
 if raw:=os.Getenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD");raw!=""{v,e:=strconv.Atoi(raw);if e!=nil||v<1{return Config{},fmt.Errorf("invalid DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD: %q",raw)};cfg.FailureThreshold=v}
 return cfg,nil
}

func NewFromEnvironment(httpClient *http.Client)(*Service,error){
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
 transactionStore,e:=routing.NewJSONFileTransactionStore(cfg.TransactionStorePath);if e!=nil{return nil,e}
 stateStore:=operational.NewProviderStateStore()
 for _, name:=range registry.Names(){state,e:=operational.NewProviderState(name);if e!=nil{return nil,e};state.Capabilities=[]operational.Capability{operational.CapabilityPPOB,operational.CapabilityBalance,operational.CapabilityWebhook};if e=stateStore.Put(state);e!=nil{return nil,e}}
 router,e:=routing.NewWithCatalogAndState(registry,store,nil,catalogStore,stateStore);if e!=nil{return nil,e}
 purchaseService,e:=routing.NewServiceWithStore(router,transactionStore);if e!=nil{return nil,e}
 return &Service{syncService:syncService,purchaseService:purchaseService,catalogSync:catalogSync,providerState:stateStore,interval:cfg.SyncInterval,catalogInterval:cfg.CatalogSyncInterval},nil
}

func New(syncService *operational.SyncService,interval time.Duration)(*Service,error){if syncService==nil{return nil,errors.New("sync service is required")};if interval<=0{return nil,errors.New("sync interval must be greater than zero")};return &Service{syncService:syncService,interval:interval},nil}
func (s *Service) Run(ctx context.Context)error{if ctx==nil{return errors.New("context is required")};if s.catalogSync==nil{return s.syncService.Run(ctx,s.interval)};_=s.catalogSync.SyncAll(ctx);ticker:=time.NewTicker(s.catalogInterval);defer ticker.Stop();go func(){_=s.syncService.Run(ctx,s.interval)}();for{select{case<-ctx.Done():return ctx.Err();case<-ticker.C:_=s.catalogSync.SyncAll(ctx)}}}
func (s *Service) PurchaseService()*routing.Service{if s==nil{return nil};return s.purchaseService}
