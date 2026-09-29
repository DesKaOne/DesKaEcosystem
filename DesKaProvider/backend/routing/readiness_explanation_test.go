package routing
import("context";"testing";"time";provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider";mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock";"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational";"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog")
func testReadinessRouter(t *testing.T,status provider.CapabilityStatus,lifecycle operational.Lifecycle,cat *catalog.Snapshot)*Router{t.Helper();r:=provider.NewRegistry();if err:=r.RegisterWithCapabilities("mock",mock.New(mock.Config{Products:[]provider.Product{{Code:"xld10"}}}),provider.CapabilityDescriptor{Capabilities:map[provider.Capability]provider.CapabilityStatus{provider.CapabilityPPOB:status}});err!=nil{t.Fatal(err)};states:=operational.NewProviderStateStore();d,_:=r.Capabilities("mock");if err:=states.Put(operational.ProviderState{ProviderName:"mock",Lifecycle:lifecycle,Capabilities:[]operational.Capability{operational.CapabilityPPOB},CapabilityFingerprint:operational.CapabilityMetadataFingerprint(d)});err!=nil{t.Fatal(err)};store:=operational.NewMemoryStore();if err:=store.Put(operational.Snapshot{ProviderName:"mock",Balance:100000,Currency:"IDR",Health:operational.HealthHealthy,LastCheckedAt:time.Now()});err!=nil{t.Fatal(err)};var cs catalog.Store;if cat!=nil{c:=catalog.NewMemoryStore();if err:=c.Put(*cat);err!=nil{t.Fatal(err)};cs=c};router,err:=NewWithCatalogAndStateAndOperationalMaxAge(r,store,nil,cs,states,time.Hour);if err!=nil{t.Fatal(err)};return router}
func reason(e ProviderRouteExplanation,c ReadinessReasonCode)(bool,bool){for _,v:=range e.Reasons{if v.Code==c{return true,v.Blocking}};return false,false}
func TestExplainProviderRouteSeparatesReadinessFromRouting(t *testing.T){r:=testReadinessRouter(t,provider.CapabilityStatus{AdapterImplemented:true,Enabled:true},operational.LifecycleEnabled,&catalog.Snapshot{ProviderName:"mock",Products:[]provider.Product{{Code:"xld10"}},SyncedAt:time.Now()});e,err:=ExplainProviderRoute(context.Background(),r,"mock",provider.CapabilityPPOB,"xld10",100);if err!=nil{t.Fatal(err)};if !e.RouteEligible{t.Fatalf("explainability changed routing: %#v",e)};for _,c:=range []ReadinessReasonCode{ReasonConfigurationMissing,ReasonTestsNotVerified,ReasonLiveValidationMissing,ReasonProductionReadinessMissing}{found,blocking:=reason(e,c);if !found||blocking{t.Fatalf("expected non-blocking %q: %#v",c,e)}}}
func TestExplainProviderRouteReportsDriftAndStaleCatalog(t *testing.T){r:=testReadinessRouter(t,provider.CapabilityStatus{AdapterImplemented:true,Enabled:true,Tested:true},operational.LifecycleEnabled,&catalog.Snapshot{ProviderName:"mock",Products:[]provider.Product{{Code:"xld10"}},SyncedAt:time.Now().Add(-2*time.Hour)});s:=r.ProviderState;st,_:=s.Get("mock");st.CapabilityFingerprint="stale";if err:=s.Put(st);err!=nil{t.Fatal(err)};e,err:=ExplainProviderRoute(context.Background(),r,"mock",provider.CapabilityPPOB,"xld10",100);if err!=nil{t.Fatal(err)};for _,c:=range []ReadinessReasonCode{ReasonCapabilityDrift,ReasonCatalogStale}{found,blocking:=reason(e,c);if !found||!blocking{t.Fatalf("expected blocking %q: %#v",c,e)}};if e.RouteEligible{t.Fatal("drift/stale catalog must block routing")}}
func TestExplainProviderRouteReasonsAreDeterministic(t *testing.T){r:=testReadinessRouter(t,provider.CapabilityStatus{AdapterImplemented:true,Enabled:false},operational.LifecycleDisabled,nil);a,_:=ExplainProviderRoute(context.Background(),r,"mock",provider.CapabilityPPOB,"",0);b,_:=ExplainProviderRoute(context.Background(),r,"mock",provider.CapabilityPPOB,"",0);if len(a.Reasons)!=len(b.Reasons){t.Fatalf("reason count changed: %#v %#v",a,b)};for i:=range a.Reasons{if a.Reasons[i]!=b.Reasons[i]{t.Fatalf("reasons changed: %#v %#v",a,b)}}}

func TestExplainProviderRouteMatchesOperationalCapabilityGate(t *testing.T) {
	router := testReadinessRouter(t, provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true}, operational.LifecycleEnabled, &catalog.Snapshot{
		ProviderName: "mock",
		Products: []provider.Product{{Code: "xld10"}},
		SyncedAt:    time.Now(),
	})
	state, ok := router.ProviderState.Get("mock")
	if !ok {
		t.Fatal("expected provider state")
	}
	state.Capabilities = []operational.Capability{operational.CapabilityBalance}
	if err := router.ProviderState.Put(state); err != nil {
		t.Fatal(err)
	}

	explanation, err := ExplainProviderRoute(context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	found, blocking := reason(explanation, ReasonOperationalCapabilityMissing)
	if !found || !blocking {
		t.Fatalf("expected blocking operational capability reason: %#v", explanation)
	}
	if explanation.RouteEligible {
		t.Fatalf("explanation must reject the same operational capability gate as Router.Select: %#v", explanation)
	}
	if _, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100}); !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("Router.Select should reject the provider at the same gate, got %v", err)
	}
}
