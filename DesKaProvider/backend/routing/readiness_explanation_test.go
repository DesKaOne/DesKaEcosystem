package routing
import("context";"errors";"reflect";"testing";"time";provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider";mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock";"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational";"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog")
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


func TestExplainProviderRouteDeepStateMatrix(t *testing.T) {
	t.Run("operational-state-missing", func(t *testing.T) {
		r := testReadinessRouter(t, provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			operational.LifecycleEnabled, &catalog.Snapshot{
				ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: time.Now(),
			})
		r.ProviderState = operational.NewProviderStateStore()

		explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
		if err != nil {
			t.Fatal(err)
		}
		found, blocking := reason(explanation, ReasonOperationalStateMissing)
		if !found || !blocking {
			t.Fatalf("expected blocking operational-state-missing reason: %#v", explanation)
		}
		if explanation.RouteEligible {
			t.Fatalf("missing operational state must block route: %#v", explanation)
		}
	})

	t.Run("adapter-not-implemented", func(t *testing.T) {
		r := testReadinessRouter(t, provider.CapabilityStatus{Enabled: false, Tested: true},
			operational.LifecycleEnabled, &catalog.Snapshot{
				ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: time.Now(),
			})
		explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
		if err != nil {
			t.Fatal(err)
		}
		found, blocking := reason(explanation, ReasonAdapterNotImplemented)
		if !found || !blocking {
			t.Fatalf("expected blocking adapter-not-implemented reason: %#v", explanation)
		}
		if explanation.RouteEligible {
			t.Fatalf("unimplemented adapter must block route: %#v", explanation)
		}
	})

	t.Run("catalog-missing", func(t *testing.T) {
		r := testReadinessRouter(t, provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true},
			operational.LifecycleEnabled, &catalog.Snapshot{
				ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: time.Now(),
			})
		r.Catalog = catalog.NewMemoryStore()

		explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
		if err != nil {
			t.Fatal(err)
		}
		found, blocking := reason(explanation, ReasonCatalogMissing)
		if !found || !blocking {
			t.Fatalf("expected blocking catalog-missing reason: %#v", explanation)
		}
		if explanation.RouteEligible {
			t.Fatalf("missing catalog must block route: %#v", explanation)
		}
	})

	t.Run("reason-order-is-canonical-with-mixed-blocking-and-nonblocking", func(t *testing.T) {
		r := testReadinessRouter(t, provider.CapabilityStatus{
			AdapterImplemented: true,
			Enabled:            false,
			Tested:             false,
			LiveTested:         false,
			ProductionReady:   false,
		}, operational.LifecycleDisabled, &catalog.Snapshot{
			ProviderName: "mock",
			Products:     []provider.Product{{Code: "other"}},
			SyncedAt:     time.Now().Add(-2 * time.Hour),
		})
		r.Now = time.Now
		reader := r.OperationalInput.(*StoreOperationalInputReader)
		now := r.NowTime()
		if err := reader.Store.Put(operational.Snapshot{
			ProviderName: "mock",
			Balance:      1,
			Currency:     "IDR",
			Health:       operational.HealthUnhealthy,
			LastCheckedAt: now.Add(-2 * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}

		explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
		if err != nil {
			t.Fatal(err)
		}
		if explanation.RouteEligible {
			t.Fatalf("mixed blocking state must be ineligible: %#v", explanation)
		}
		for i := 1; i < len(explanation.Reasons); i++ {
			if explanation.Reasons[i-1].Code > explanation.Reasons[i].Code {
				t.Fatalf("reasons are not canonically ordered: %#v", explanation.Reasons)
			}
		}
		for _, code := range []ReadinessReasonCode{
			ReasonCapabilityDisabled,
			ReasonCatalogStale,
			ReasonProductUnavailable,
			ReasonOperationalHealthUnhealthy,
			ReasonInsufficientBalance,
			ReasonOperationalSnapshotStale,
		} {
			found, blocking := reason(explanation, code)
			if !found || !blocking {
				t.Fatalf("expected blocking mixed-state reason %q: %#v", code, explanation)
			}
		}
		for _, code := range []ReadinessReasonCode{
			ReasonConfigurationMissing,
			ReasonTestsNotVerified,
			ReasonLiveValidationMissing,
			ReasonProductionReadinessMissing,
		} {
			found, blocking := reason(explanation, code)
			if !found || blocking {
				t.Fatalf("expected non-blocking readiness reason %q: %#v", code, explanation)
			}
		}
	})
}


func TestExplainProviderRouteTransitionParityWithRouterSelect(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	router := testReadinessRouter(t, provider.CapabilityStatus{
		AdapterImplemented: true,
		Enabled:            true,
		Tested:             true,
	}, operational.LifecycleEnabled, &catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "xld10"}},
		SyncedAt:     now,
	})
	router.Now = func() time.Time { return now }

	reader, ok := router.OperationalInput.(*StoreOperationalInputReader)
	if !ok {
		t.Fatalf("expected store-backed operational input reader, got %T", router.OperationalInput)
	}
	if err := reader.Store.Put(operational.Snapshot{
		ProviderName:       "mock",
		Balance:            100000,
		Currency:           "IDR",
		Health:             operational.HealthHealthy,
		LastCheckedAt:      now,
		LastSuccessAt:      now,
		ConsecutiveFailures: 0,
	}); err != nil {
		t.Fatal(err)
	}

	assertParity := func(label string, expectEligible bool, expectedReason ReadinessReasonCode) {
		t.Helper()
		explanation, err := ExplainProviderRoute(context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100)
		if err != nil {
			t.Fatal(err)
		}
		selected, selectErr := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
		if expectEligible {
			if !explanation.RouteEligible {
				t.Fatalf("%s: explanation unexpectedly blocked: %#v", label, explanation)
			}
			if selectErr != nil || selected != "mock" {
				t.Fatalf("%s: Router.Select unexpectedly rejected route: provider=%q err=%v", label, selected, selectErr)
			}
			return
		}
		if explanation.RouteEligible {
			t.Fatalf("%s: explanation unexpectedly eligible: %#v", label, explanation)
		}
		found, blocking := reason(explanation, expectedReason)
		if !found || !blocking {
			t.Fatalf("%s: expected blocking reason %q: %#v", label, expectedReason, explanation)
		}
		if !errors.Is(selectErr, ErrNoProviderAvailable) {
			t.Fatalf("%s: Router.Select unexpectedly selected provider: provider=%q err=%v", label, selected, selectErr)
		}
	}

	assertParity("initial", true, "")
	state, ok := router.ProviderState.Get("mock")
	if !ok {
		t.Fatal("expected provider state")
	}
	state.Lifecycle = operational.LifecycleDisabled
	if err := router.ProviderState.Put(state); err != nil {
		t.Fatal(err)
	}
	assertParity("lifecycle-disabled", false, ReasonLifecycleDisabled)

	state.Lifecycle = operational.LifecycleEnabled
	state.CapabilityFingerprint = "drifted"
	if err := router.ProviderState.Put(state); err != nil {
		t.Fatal(err)
	}
	assertParity("capability-drift", false, ReasonCapabilityDrift)

	admin, err := operational.NewProviderAdminService(router.ProviderState)
	if err != nil {
		t.Fatal(err)
	}
	reconciled, err := admin.ReconcileCapabilityState("mock", router.Registry)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Drifted || reconciled.State.Enabled() {
		t.Fatalf("reconciliation must clear drift without re-enabling lifecycle: %#v", reconciled)
	}
	assertParity("reconciled-but-disabled", false, ReasonLifecycleDisabled)

	if _, err := admin.Enable("mock"); err != nil {
		t.Fatal(err)
	}
	assertParity("explicitly-enabled", true, "")
}


func TestExplainProviderRouteReturnedReasonsAreCopyIsolated(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	r := testReadinessRouter(t, provider.CapabilityStatus{
		AdapterImplemented: true,
		Enabled: true,
		Tested: true,
	}, operational.LifecycleEnabled, &catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "xld10"}},
		SyncedAt:     now,
	})
	r.Now = func() time.Time { return now }

	first, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	original := append([]ReadinessReason(nil), first.Reasons...)
	if len(first.Reasons) == 0 {
		t.Fatal("expected explanation reasons")
	}

	first.Reasons[0] = ReadinessReason{Code: ReadinessReasonCode("caller_mutated"), Blocking: true}
	first.Reasons = append(first.Reasons, ReadinessReason{Code: ReadinessReasonCode("caller_appended"), Blocking: true})

	second, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Reasons) != len(original) {
		t.Fatalf("caller mutation changed subsequent explanation length: got %d want %d", len(second.Reasons), len(original))
	}
	for i := range original {
		if second.Reasons[i] != original[i] {
			t.Fatalf("caller mutation leaked into subsequent explanation: got %#v want %#v", second.Reasons, original)
		}
	}
}

func TestExplainProviderRouteSnapshotResultIsolatedFromLaterStateChanges(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	r := testReadinessRouter(t, provider.CapabilityStatus{
		AdapterImplemented: true,
		Enabled: true,
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
		ProviderName:  "mock",
		Balance:       100000,
		Currency:      "IDR",
		Health:        operational.HealthHealthy,
		LastCheckedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	before, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	if !before.RouteEligible {
		t.Fatalf("expected initial route eligibility: %#v", before)
	}

	if err := reader.Store.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       50,
		Currency:      "IDR",
		Health:        operational.HealthUnhealthy,
		LastCheckedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	catalogStore, ok := r.Catalog.(catalog.Store)
	if !ok {
		t.Fatalf("expected catalog store")
	}
	if err := catalogStore.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "other"}},
		SyncedAt:     now,
	}); err != nil {
		t.Fatal(err)
	}

	if !before.RouteEligible {
		t.Fatalf("previous explanation was mutated by later source changes: %#v", before)
	}
	after, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	if after.RouteEligible {
		t.Fatalf("new explanation must observe changed operational/catalog state: %#v", after)
	}
	for _, code := range []ReadinessReasonCode{ReasonOperationalHealthUnhealthy, ReasonInsufficientBalance, ReasonProductUnavailable} {
		found, blocking := reason(after, code)
		if !found || !blocking {
			t.Fatalf("expected blocking reason %q after state change: %#v", code, after)
		}
	}
}

func TestExplainProviderRouteRepeatedCallsRemainDeterministicAfterCallerMutation(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	r := testReadinessRouter(t, provider.CapabilityStatus{
		AdapterImplemented: true,
		Enabled: false,
		Tested: false,
	}, operational.LifecycleDisabled, &catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "other"}},
		SyncedAt:     now.Add(-2 * time.Hour),
	})
	r.Now = func() time.Time { return now }

	first, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	first.Reasons = append(first.Reasons, ReadinessReason{
		Code:    ReadinessReasonCode("caller_mutated"),
		Blocking: true,
	})

	second, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	third, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, third) {
		t.Fatalf("repeated explanations diverged after caller mutation: %#v %#v", second, third)
	}
	for _, rr := range second.Reasons {
		if rr.Code == ReadinessReasonCode("caller_mutated") {
			t.Fatalf("caller-mutated reason leaked into subsequent explanation: %#v", second)
		}
	}
}


func TestExplainProviderRouteBlockingReasonsMapToRouterErrors(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		mutate func(*Router)
		reason ReadinessReasonCode
		routerErr error
	}{
		{
			name: "lifecycle-disabled",
			mutate: func(r *Router) {
				s, _ := r.ProviderState.Get("mock")
				s.Lifecycle = operational.LifecycleDisabled
				if err := r.ProviderState.Put(s); err != nil { t.Fatal(err) }
			},
			reason: ReasonLifecycleDisabled,
			routerErr: ErrNoProviderAvailable,
		},
		{
			name: "capability-drift",
			mutate: func(r *Router) {
				s, _ := r.ProviderState.Get("mock")
				s.CapabilityFingerprint = "drifted"
				if err := r.ProviderState.Put(s); err != nil { t.Fatal(err) }
			},
			reason: ReasonCapabilityDrift,
			routerErr: ErrProviderCapabilityDrift,
		},
		{
			name: "operational-stale",
			mutate: func(r *Router) {
				reader := r.OperationalInput.(*StoreOperationalInputReader)
				if err := reader.Store.Put(operational.Snapshot{
					ProviderName: "mock", Balance: 100000, Currency: "IDR",
					Health: operational.HealthHealthy, LastCheckedAt: now.Add(-2 * time.Hour),
				}); err != nil { t.Fatal(err) }
				r.Now = func() time.Time { return now }
			},
			reason: ReasonOperationalSnapshotStale,
			routerErr: ErrOperationalSnapshotStale,
		},
		{
			name: "catalog-stale",
			mutate: func(r *Router) {
				r.Now = func() time.Time { return now.Add(2 * time.Hour) }
				reader := r.OperationalInput.(*StoreOperationalInputReader)
				if err := reader.Store.Put(operational.Snapshot{
					ProviderName: "mock", Balance: 100000, Currency: "IDR",
					Health: operational.HealthHealthy, LastCheckedAt: now.Add(2 * time.Hour),
				}); err != nil { t.Fatal(err) }
			},
			reason: ReasonCatalogStale,
			routerErr: ErrCatalogStale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReadinessRouter(t, provider.CapabilityStatus{
				AdapterImplemented: true, Enabled: true, Tested: true,
			}, operational.LifecycleEnabled, &catalog.Snapshot{
				ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now,
			})
			r.Now = func() time.Time { return now }
			reader := r.OperationalInput.(*StoreOperationalInputReader)
			if err := reader.Store.Put(operational.Snapshot{
				ProviderName: "mock", Balance: 100000, Currency: "IDR",
				Health: operational.HealthHealthy, LastCheckedAt: now,
			}); err != nil { t.Fatal(err) }
			tt.mutate(r)

			explanation, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
			if err != nil { t.Fatal(err) }
			found, blocking := reason(explanation, tt.reason)
			if !found || !blocking {
				t.Fatalf("expected blocking explanation reason %q: %#v", tt.reason, explanation)
			}
			_, selectErr := r.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
			if !errors.Is(selectErr, tt.routerErr) {
				t.Fatalf("expected Router.Select error %v for explanation reason %q, got %v", tt.routerErr, tt.reason, selectErr)
			}
		})
	}
}

func TestExplainProviderRouteCompoundBlockingReasonsMatchJoinedRouterErrors(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	registry := provider.NewRegistry()
	if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10"}}}), provider.CapabilityDescriptor{
		Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: {AdapterImplemented: true, Enabled: true, Tested: true},
		},
	}); err != nil { t.Fatal(err) }
	if err := registry.RegisterWithCapabilities("mock2", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10"}}}), provider.CapabilityDescriptor{
		Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: {AdapterImplemented: true, Enabled: true, Tested: true},
		},
	}); err != nil { t.Fatal(err) }

	states := operational.NewProviderStateStore()
	for _, name := range []string{"mock", "mock2"} {
		d, err := registry.Capabilities(name)
		if err != nil { t.Fatal(err) }
		if err := states.Put(operational.ProviderState{
			ProviderName: name,
			Lifecycle: operational.LifecycleEnabled,
			Capabilities: []operational.Capability{operational.CapabilityPPOB},
			CapabilityFingerprint: operational.CapabilityMetadataFingerprint(d),
		}); err != nil { t.Fatal(err) }
	}

	store := operational.NewMemoryStore()
	for _, name := range []string{"mock", "mock2"} {
		if err := store.Put(operational.Snapshot{
			ProviderName: name, Balance: 100000, Currency: "IDR",
			Health: operational.HealthHealthy, LastCheckedAt: now,
		}); err != nil { t.Fatal(err) }
	}

	catalogStore := catalog.NewMemoryStore()
	for _, name := range []string{"mock", "mock2"} {
		if err := catalogStore.Put(catalog.Snapshot{
			ProviderName: name, Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now,
		}); err != nil { t.Fatal(err) }
	}

	r, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogStore, states, time.Hour)
	if err != nil { t.Fatal(err) }
	r.Now = func() time.Time { return now.Add(2 * time.Hour) }
	for _, name := range []string{"mock", "mock2"} {
		if err := store.Put(operational.Snapshot{
			ProviderName: name, Balance: 100000, Currency: "IDR",
			Health: operational.HealthHealthy, LastCheckedAt: now.Add(2 * time.Hour),
		}); err != nil { t.Fatal(err) }
	}

	state, _ := r.ProviderState.Get("mock")
	state.CapabilityFingerprint = "drifted"
	if err := r.ProviderState.Put(state); err != nil { t.Fatal(err) }

	explanationDrift, err := ExplainProviderRoute(context.Background(), r, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil { t.Fatal(err) }
	found, blocking := reason(explanationDrift, ReasonCapabilityDrift)
	if !found || !blocking { t.Fatalf("expected provider mock drift blocker: %#v", explanationDrift) }

	explanationCatalog, err := ExplainProviderRoute(context.Background(), r, "mock2", provider.CapabilityPPOB, "xld10", 100)
	if err != nil { t.Fatal(err) }
	found, blocking = reason(explanationCatalog, ReasonCatalogStale)
	if !found || !blocking { t.Fatalf("expected provider mock2 catalog-stale blocker: %#v", explanationCatalog) }

	_, selectErr := r.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
	if !errors.Is(selectErr, ErrNoProviderAvailable) ||
		!errors.Is(selectErr, ErrProviderCapabilityDrift) ||
		!errors.Is(selectErr, ErrCatalogStale) {
		t.Fatalf("Router.Select joined error semantics diverged from aggregated explanation blockers: %v", selectErr)
	}
}

func TestExplainProviderRouteAggregateReasonsMatchRouterJoinedErrorGates(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	registry := provider.NewRegistry()
	for _, name := range []string{"drift", "operational-stale", "catalog-stale", "healthy-blocked"} {
		if err := registry.RegisterWithCapabilities(name, mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10"}},
		}), provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: {AdapterImplemented: true, Enabled: true, Tested: true},
		}}); err != nil {
			t.Fatal(err)
		}
	}

	states := operational.NewProviderStateStore()
	store := operational.NewMemoryStore()
	catalogStore := catalog.NewMemoryStore()
	for _, name := range []string{"drift", "operational-stale", "catalog-stale", "healthy-blocked"} {
		d, err := registry.Capabilities(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := states.Put(operational.ProviderState{
			ProviderName:          name,
			Lifecycle:             operational.LifecycleEnabled,
			Capabilities:          []operational.Capability{operational.CapabilityPPOB},
			CapabilityFingerprint: operational.CapabilityMetadataFingerprint(d),
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.Put(operational.Snapshot{
			ProviderName: name, Balance: 100000, Currency: "IDR",
			Health: operational.HealthHealthy, LastCheckedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		syncedAt := now
		if name == "catalog-stale" {
			syncedAt = now.Add(-2 * time.Hour)
		}
		if err := catalogStore.Put(catalog.Snapshot{
			ProviderName: name, Products: []provider.Product{{Code: "xld10"}}, SyncedAt: syncedAt,
		}); err != nil {
			t.Fatal(err)
		}
	}

	r, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogStore, states, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	r.Now = func() time.Time { return now }

	// Drift is an early routing gate. Its stale operational snapshot must not
	// contribute an operational-stale aggregate error for the same provider.
	driftState, _ := states.Get("drift")
	driftState.CapabilityFingerprint = "drifted"
	if err := states.Put(driftState); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(operational.Snapshot{
		ProviderName: "drift", Balance: 100000, Currency: "IDR",
		Health: operational.HealthHealthy, LastCheckedAt: now.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// Operational stale is an earlier gate than catalog evaluation. Its fresh
	// catalog must therefore not contribute catalog-stale for this provider.
	if err := store.Put(operational.Snapshot{
		ProviderName: "operational-stale", Balance: 100000, Currency: "IDR",
		Health: operational.HealthHealthy, LastCheckedAt: now.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// This provider reaches product availability but has insufficient balance;
	// the current Router.Select contract does not aggregate a dedicated
	// insufficient-balance sentinel, so explanation evidence must remain
	// observational and must not invent one.
	if err := store.Put(operational.Snapshot{
		ProviderName: "healthy-blocked", Balance: 1, Currency: "IDR",
		Health: operational.HealthHealthy, LastCheckedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	type expected struct {
		name   string
		reason ReadinessReasonCode
	}
	for _, tt := range []expected{
		{name: "drift", reason: ReasonCapabilityDrift},
		{name: "operational-stale", reason: ReasonOperationalSnapshotStale},
		{name: "catalog-stale", reason: ReasonCatalogStale},
		{name: "healthy-blocked", reason: ReasonInsufficientBalance},
	} {
		explanation, err := ExplainProviderRoute(context.Background(), r, tt.name, provider.CapabilityPPOB, "xld10", 100)
		if err != nil {
			t.Fatal(err)
		}
		found, blocking := reason(explanation, tt.reason)
		if !found || !blocking {
			t.Fatalf("%s: expected blocking explanation reason %q: %#v", tt.name, tt.reason, explanation)
		}
	}

	driftExplanation, _ := ExplainProviderRoute(context.Background(), r, "drift", provider.CapabilityPPOB, "xld10", 100)
	if found, _ := reason(driftExplanation, ReasonOperationalSnapshotStale); found {
		t.Fatalf("drift provider must not report later operational-stale gate: %#v", driftExplanation)
	}
	operationalExplanation, _ := ExplainProviderRoute(context.Background(), r, "operational-stale", provider.CapabilityPPOB, "xld10", 100)
	if found, _ := reason(operationalExplanation, ReasonCatalogStale); found {
		t.Fatalf("operational-stale provider must not report later catalog-stale gate: %#v", operationalExplanation)
	}

	_, selectErr := r.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
	if !errors.Is(selectErr, ErrNoProviderAvailable) ||
		!errors.Is(selectErr, ErrOperationalSnapshotStale) ||
		!errors.Is(selectErr, ErrCatalogStale) {
		t.Fatalf("aggregate router errors missing expected joined sentinels: %v", selectErr)
	}
	if errors.Is(selectErr, ErrProviderCapabilityDrift) {
		// The drift provider is intentionally also operational-stale, but Router.Select
		// stops at capability drift, so capability drift must still be represented.
	} else {
		t.Fatalf("aggregate router errors missing capability-drift sentinel: %v", selectErr)
	}
}
