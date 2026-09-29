package routing

import (
 "context"
 "errors"
 "sort"
 "time"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

type ReadinessReasonCode string
const (
 ReasonProviderNotRegistered ReadinessReasonCode="provider_not_registered"
 ReasonOperationalStateMissing ReadinessReasonCode="operational_state_missing"
 ReasonOperationalCapabilityMissing ReadinessReasonCode="operational_capability_missing"
 ReasonLifecycleDisabled ReadinessReasonCode="lifecycle_disabled"
 ReasonCapabilityNotDeclared ReadinessReasonCode="capability_not_declared"
 ReasonAdapterNotImplemented ReadinessReasonCode="adapter_not_implemented"
 ReasonConfigurationMissing ReadinessReasonCode="configuration_missing"
 ReasonTestsNotVerified ReadinessReasonCode="tests_not_verified"
 ReasonCapabilityDisabled ReadinessReasonCode="capability_disabled"
 ReasonLiveValidationMissing ReadinessReasonCode="live_validation_missing"
 ReasonProductionReadinessMissing ReadinessReasonCode="production_readiness_missing"
 ReasonCapabilityDrift ReadinessReasonCode="capability_drift"
 ReasonOperationalSnapshotStale ReadinessReasonCode="operational_snapshot_stale"
 ReasonOperationalHealthUnhealthy ReadinessReasonCode="operational_health_unhealthy"
 ReasonInsufficientBalance ReadinessReasonCode="insufficient_balance"
 ReasonCatalogMissing ReadinessReasonCode="catalog_missing"
 ReasonCatalogStale ReadinessReasonCode="catalog_stale"
 ReasonProductUnavailable ReadinessReasonCode="product_unavailable"
)
type ReadinessReason struct { Code ReadinessReasonCode; Blocking bool }
type ProviderRouteExplanation struct { ProviderName string; Capability provider.Capability; RouteEligible bool; Reasons []ReadinessReason }

func ExplainProviderRoute(ctx context.Context, router *Router, name string, capability provider.Capability, productCode string, amount int64) (ProviderRouteExplanation,error) {
 if router==nil||router.Registry==nil{return ProviderRouteExplanation{},errors.New("provider router is required")}
 if ctx==nil{return ProviderRouteExplanation{},errors.New("context is required")}
 name=normalize(name); result:=ProviderRouteExplanation{ProviderName:name,Capability:capability}
 if name=="" { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonProviderNotRegistered,Blocking:true}); return result,nil }
 if _,err:=router.Registry.Get(name);err!=nil { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonProviderNotRegistered,Blocking:true}); return result,nil }
 descriptor,err:=router.Registry.Capabilities(name); if err!=nil { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonProviderNotRegistered,Blocking:true}); return result,nil }
 status,declared:=descriptor.Status(capability)
 var state operational.ProviderState
 if router.ProviderState!=nil {
  var ok bool; state,ok=router.ProviderState.Get(name)
  if !ok { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonOperationalStateMissing,Blocking:true}) } else { if !state.Enabled() { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonLifecycleDisabled,Blocking:true}) }; if !state.Supports(capability) { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonOperationalCapabilityMissing,Blocking:true}) } }
 }
 if !declared { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonCapabilityNotDeclared,Blocking:true}) } else {
  if !status.AdapterImplemented { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonAdapterNotImplemented,Blocking:true}) }
  if !status.Configured { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonConfigurationMissing}) }
  if !status.Tested { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonTestsNotVerified}) }
  if !status.Enabled { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonCapabilityDisabled,Blocking:true}) }
  if !status.LiveTested { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonLiveValidationMissing}) }
  if !status.ProductionReady { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonProductionReadinessMissing}) }
  if router.ProviderState!=nil { if _,ok:=router.ProviderState.Get(name);ok { if operational.DetectCapabilityDrift(state,descriptor).Drifted(){result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonCapabilityDrift,Blocking:true})} } }
 }
 if router.OperationalInput==nil { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonOperationalStateMissing,Blocking:true}) } else {
  input,err:=router.OperationalInput.ReadOperationalInput(ctx,name,router.OperationalMaxAge,router.NowTime())
  if err==nil { snap:=input.Snapshot; if router.OperationalMaxAge>0&&!isFresh(snap.LastCheckedAt,router.NowTime(),router.OperationalMaxAge){result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonOperationalSnapshotStale,Blocking:true})}; if snap.Health!=operational.HealthHealthy{result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonOperationalHealthUnhealthy,Blocking:true})}; if amount>0&&snap.Balance<amount{result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonInsufficientBalance,Blocking:true})} } else if errors.Is(err,operational.ErrOperationalSnapshotNotFound){result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonOperationalStateMissing,Blocking:true})}
 }
 if router.Catalog!=nil { snap,ok:=router.Catalog.Get(name); if !ok { result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonCatalogMissing,Blocking:true}) } else { if router.CatalogMaxAge>0&&!isFresh(snap.SyncedAt,router.NowTime(),router.CatalogMaxAge){result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonCatalogStale,Blocking:true})}; if productCode!=""&&!hasProduct(snap.Products,productCode){result.Reasons=append(result.Reasons,ReadinessReason{Code:ReasonProductUnavailable,Blocking:true})} } }
 result.Reasons=uniqueSortedReasons(result.Reasons); result.RouteEligible=descriptor.Supports(capability); for _,reason:=range result.Reasons{if reason.Blocking{result.RouteEligible=false;break}}; return result,nil
}
func (r *Router) NowTime() time.Time {if r!=nil&&r.Now!=nil{return r.Now()};return time.Now()}
func uniqueSortedReasons(reasons []ReadinessReason)[]ReadinessReason{seen:=map[ReadinessReasonCode]struct{}{};out:=make([]ReadinessReason,0,len(reasons));for _,reason:=range reasons{if _,ok:=seen[reason.Code];ok{continue};seen[reason.Code]=struct{}{};out=append(out,reason)};sort.Slice(out,func(i,j int)bool{return string(out[i].Code)<string(out[j].Code)});return out}
