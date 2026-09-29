package routing
import("context";"errors";"testing";"time";provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider";mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock";"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational";"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog")
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


func TestExplainProviderRouteCandidateRejectionParityMatrix(t *testing.T) {
	tests := []struct {
		name    string
		status  provider.CapabilityStatus
		life    operational.Lifecycle
		cat     *catalog.Snapshot
		mutate  func(*Router)
		reason  ReadinessReasonCode
		blocking bool
	}{
		{
			name: "lifecycle-disabled",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			life: operational.LifecycleDisabled,
			cat: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: time.Now()},
			reason: ReasonLifecycleDisabled, blocking: true,
		},
		{
			name: "capability-disabled",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: false, Tested: true},
			life: operational.LifecycleEnabled,
			cat: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: time.Now()},
			reason: ReasonCapabilityDisabled, blocking: true,
		},
		{
			name: "capability-drift",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			life: operational.LifecycleEnabled,
			cat: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: time.Now()},
			mutate: func(r *Router) {
				state, _ := r.ProviderState.Get("mock")
				state.CapabilityFingerprint = "drifted"
				if err := r.ProviderState.Put(state); err != nil { t.Fatal(err) }
			},
			reason: ReasonCapabilityDrift, blocking: true,
		},
		{
			name: "catalog-stale",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			life: operational.LifecycleEnabled,
			cat: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: time.Now().Add(-2 * time.Hour)},
			reason: ReasonCatalogStale, blocking: true,
		},
		{
			name: "product-unavailable",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			life: operational.LifecycleEnabled,
			cat: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "other"}}, SyncedAt: time.Now()},
			reason: ReasonProductUnavailable, blocking: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReadinessRouter(t, tt.status, tt.life, tt.cat)
			if tt.mutate != nil { tt.mutate(r) }
			e, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
			if err != nil { t.Fatal(err) }
			found, blocking := reason(e, tt.reason)
			if !found || blocking != tt.blocking {
				t.Fatalf("expected reason %q blocking=%v, got %#v", tt.reason, tt.blocking, e)
			}
			if !e.RouteEligible {
				return
			}
			for _, rr := range e.Reasons {
				if rr.Blocking {
					t.Fatalf("route explanation marked eligible despite blocking reason: %#v", e)
				}
			}
		})
	}
}


func TestExplainProviderRouteOperationalFreshnessHealthBalanceParity(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		lastChecked time.Time
		health     operational.Health
		balance    int64
		reason     ReadinessReasonCode
		expectStale bool
	}{
		{
			name:        "operational-freshness",
			lastChecked: now.Add(-2 * time.Hour),
			health:      operational.HealthHealthy,
			balance:     100000,
			reason:      ReasonOperationalSnapshotStale,
			expectStale: true,
		},
		{
			name:        "operational-health",
			lastChecked: now,
			health:      operational.HealthUnhealthy,
			balance:     100000,
			reason:      ReasonOperationalHealthUnhealthy,
		},
		{
			name:        "operational-balance",
			lastChecked: now,
			health:      operational.HealthHealthy,
			balance:     50,
			reason:      ReasonInsufficientBalance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReadinessRouter(t, provider.CapabilityStatus{
				AdapterImplemented: true,
				Enabled:            true,
				Tested:             true,
			}, operational.LifecycleEnabled, &catalog.Snapshot{
				ProviderName: "mock",
				Products:     []provider.Product{{Code: "xld10"}},
				SyncedAt:     now,
			})
			r.Now = func() time.Time { return now }

			reader, ok := r.OperationalInput.(*StoreOperationalInputReader)
			if !ok {
				t.Fatalf("expected store-backed operational input reader, got %T", r.OperationalInput)
			}
			if err := reader.Store.Put(operational.Snapshot{
				ProviderName:    "mock",
				Balance:         tt.balance,
				Currency:        "IDR",
				Health:          tt.health,
				LastCheckedAt:   tt.lastChecked,
				LastSuccessAt:   tt.lastChecked,
				ConsecutiveFailures: 0,
			}); err != nil {
				t.Fatal(err)
			}

			explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
			if err != nil {
				t.Fatal(err)
			}
			found, blocking := reason(explanation, tt.reason)
			if !found || !blocking {
				t.Fatalf("expected blocking reason %q: %#v", tt.reason, explanation)
			}
			if explanation.RouteEligible {
				t.Fatalf("explanation must reject the same operational gate as Router.Select: %#v", explanation)
			}

			_, selectErr := r.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
			if !errors.Is(selectErr, ErrNoProviderAvailable) {
				t.Fatalf("Router.Select should reject the provider at %q gate, got %v", tt.reason, selectErr)
			}
			if tt.expectStale && !errors.Is(selectErr, ErrOperationalSnapshotStale) {
				t.Fatalf("stale operational gate should be surfaced by Router.Select, got %v", selectErr)
			}
		})
	}
}


func TestRouterReadinessStatesDoNotBypassCapabilityAndOperationalGates(t *testing.T) {
	states := []struct {
		name   string
		status provider.CapabilityStatus
	}{
		{name: "implemented", status: provider.CapabilityStatus{AdapterImplemented: true}},
		{name: "tested", status: provider.CapabilityStatus{AdapterImplemented: true, Tested: true}},
		{name: "live-validated", status: provider.CapabilityStatus{AdapterImplemented: true, Tested: true, Enabled: true, LiveTested: true}},
		{name: "production-ready", status: provider.CapabilityStatus{
			Verified: true, Configured: true, AdapterImplemented: true,
			Tested: true, Enabled: true, LiveTested: true, ProductionReady: true,
		}},
	}

	for _, tc := range states {
		t.Run(tc.name+"-operational-capability-gate", func(t *testing.T) {
			r := testReadinessRouter(t, tc.status, operational.LifecycleEnabled, &catalog.Snapshot{
				ProviderName: "mock",
				Products:     []provider.Product{{Code: "xld10"}},
				SyncedAt:    time.Now(),
			})
			state, ok := r.ProviderState.Get("mock")
			if !ok {
				t.Fatal("expected provider state")
			}
			state.Capabilities = []operational.Capability{operational.CapabilityBalance}
			if err := r.ProviderState.Put(state); err != nil {
				t.Fatal(err)
			}
			explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
			if err != nil {
				t.Fatal(err)
			}
			descriptor, err := r.Registry.Capabilities("mock")
			if err != nil {
				t.Fatal(err)
			}
			if tc.status.Enabled && !descriptor.Supports(provider.CapabilityPPOB) {
				t.Fatalf("readiness state %q should remain supported by registry metadata: %#v", tc.name, descriptor)
			}
			if explanation.RouteEligible {
				t.Fatalf("readiness state %q bypassed operational capability gate: %#v", tc.name, explanation)
			}
			if _, err := r.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100}); !errors.Is(err, ErrNoProviderAvailable) {
				t.Fatalf("readiness state %q bypassed Router.Select operational capability gate: %v", tc.name, err)
			}
		})
	}

	t.Run("production-ready-does-not-bypass-operational-gate", func(t *testing.T) {
		r := testReadinessRouter(t, provider.CapabilityStatus{
			Verified: true, Configured: true, AdapterImplemented: true,
			Tested: true, Enabled: true, LiveTested: true, ProductionReady: true,
		}, operational.LifecycleEnabled, &catalog.Snapshot{
			ProviderName: "mock",
			Products:     []provider.Product{{Code: "xld10"}},
			SyncedAt:    time.Now(),
		})
		reader := r.OperationalInput.(*StoreOperationalInputReader)
		now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
		if err := reader.Store.Put(operational.Snapshot{
			ProviderName: "mock", Balance: 1, Currency: "IDR",
			Health: operational.HealthUnhealthy, LastCheckedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		r.Now = func() time.Time { return now }

		explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
		if err != nil {
			t.Fatal(err)
		}
		if explanation.RouteEligible {
			t.Fatalf("ProductionReady must not bypass operational health/balance gates: %#v", explanation)
		}
		if _, err := r.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100}); !errors.Is(err, ErrNoProviderAvailable) {
			t.Fatalf("ProductionReady bypassed Router.Select operational gate: %v", err)
		}
	})
}


func TestExplainProviderRouteParityAcrossAdministrativeRoutingStateMatrix(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		status     provider.CapabilityStatus
		lifecycle  operational.Lifecycle
		catalog    *catalog.Snapshot
		mutate     func(*Router)
		reason     ReadinessReasonCode
		shouldRoute bool
	}{
		{
			name: "recovered-and-explicitly-enabled",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			lifecycle: operational.LifecycleEnabled,
			catalog: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now},
			shouldRoute: true,
		},
		{
			name: "lifecycle-disabled",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			lifecycle: operational.LifecycleDisabled,
			catalog: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now},
			reason: ReasonLifecycleDisabled,
		},
		{
			name: "capability-drifted",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			lifecycle: operational.LifecycleEnabled,
			catalog: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now},
			mutate: func(r *Router) {
				state, _ := r.ProviderState.Get("mock")
				state.CapabilityFingerprint = "drifted"
				if err := r.ProviderState.Put(state); err != nil {
					t.Fatal(err)
				}
			},
			reason: ReasonCapabilityDrift,
		},
		{
			name: "operational-stale",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			lifecycle: operational.LifecycleEnabled,
			catalog: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now},
			mutate: func(r *Router) {
				reader := r.OperationalInput.(*StoreOperationalInputReader)
				if err := reader.Store.Put(operational.Snapshot{
					ProviderName: "mock", Balance: 100000, Currency: "IDR",
					Health: operational.HealthHealthy, LastCheckedAt: now.Add(-2 * time.Hour),
				}); err != nil {
					t.Fatal(err)
				}
				r.Now = func() time.Time { return now }
			},
			reason: ReasonOperationalSnapshotStale,
		},
		{
			name: "operational-unhealthy",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			lifecycle: operational.LifecycleEnabled,
			catalog: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now},
			mutate: func(r *Router) {
				reader := r.OperationalInput.(*StoreOperationalInputReader)
				if err := reader.Store.Put(operational.Snapshot{
					ProviderName: "mock", Balance: 100000, Currency: "IDR",
					Health: operational.HealthUnhealthy, LastCheckedAt: now,
				}); err != nil {
					t.Fatal(err)
				}
				r.Now = func() time.Time { return now }
			},
			reason: ReasonOperationalHealthUnhealthy,
		},
		{
			name: "insufficient-balance",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			lifecycle: operational.LifecycleEnabled,
			catalog: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now},
			mutate: func(r *Router) {
				reader := r.OperationalInput.(*StoreOperationalInputReader)
				if err := reader.Store.Put(operational.Snapshot{
					ProviderName: "mock", Balance: 50, Currency: "IDR",
					Health: operational.HealthHealthy, LastCheckedAt: now,
				}); err != nil {
					t.Fatal(err)
				}
				r.Now = func() time.Time { return now }
			},
			reason: ReasonInsufficientBalance,
		},
		{
			name: "catalog-stale",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			lifecycle: operational.LifecycleEnabled,
			catalog: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now.Add(-2 * time.Hour)},
			mutate: func(r *Router) { r.Now = func() time.Time { return now } },
			reason: ReasonCatalogStale,
		},
		{
			name: "product-unavailable",
			status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			lifecycle: operational.LifecycleEnabled,
			catalog: &catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "other"}}, SyncedAt: now},
			mutate: func(r *Router) { r.Now = func() time.Time { return now } },
			reason: ReasonProductUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReadinessRouter(t, tt.status, tt.lifecycle, tt.catalog)
			if tt.mutate != nil {
				tt.mutate(r)
			}

			explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
			if err != nil {
				t.Fatal(err)
			}
			selected, selectErr := r.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
			routeSucceeded := selectErr == nil

			if routeSucceeded != explanation.RouteEligible {
				t.Fatalf("administrative explanation/router parity mismatch: selected=%q err=%v explanation=%#v", selected, selectErr, explanation)
			}
			if routeSucceeded != tt.shouldRoute {
				t.Fatalf("unexpected routing result: selected=%q err=%v explanation=%#v", selected, selectErr, explanation)
			}

			if tt.reason != "" {
				found, blocking := reason(explanation, tt.reason)
				if !found || !blocking {
					t.Fatalf("expected blocking reason %q: %#v", tt.reason, explanation)
				}
			} else {
				for _, rr := range explanation.Reasons {
					if rr.Blocking {
						t.Fatalf("eligible route must have no blocking administrative reason: %#v", explanation)
					}
				}
			}
		})
	}
}
