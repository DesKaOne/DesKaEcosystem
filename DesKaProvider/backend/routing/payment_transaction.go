package routing
import (
 "errors"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
 payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)
func NewPaymentTransactionState(req payment.PaymentRequest, providerName string) (TransactionState,error) {
 if err:=payment.ValidateRequest(req); err!=nil{return TransactionState{},err}
 if providerName==""{return TransactionState{},errors.New("transaction provider name is required")}
 return TransactionState{Kind:TransactionKindPayment,Execution:PurchaseExecution{ProviderName:providerName,Result:provider.PurchaseResult{ReferenceID:req.ReferenceID,Status:provider.StatusPending}},Payment:&payment.Transaction{ReferenceID:req.ReferenceID,Amount:req.Amount,Currency:req.Currency,CustomerID:req.CustomerID,Description:req.Description,Status:payment.StatusPending},Version:1},nil
}
func validatePaymentTransactionTransition(previous,next TransactionState) error {
 if previous.Payment==nil||next.Payment==nil{return ErrReferenceConflict}
 if previous.Payment.Status==payment.StatusSuccess||previous.Payment.Status==payment.StatusFailed {
  if !samePaymentObservedResult(previous.Payment,next.Payment){return ErrReferenceConflict};return nil
 }
 if previous.Payment.Status!=payment.StatusPending{return ErrReferenceConflict}
 switch next.Payment.Status{case payment.StatusPending,payment.StatusSuccess,payment.StatusFailed:return nil;default:return ErrReferenceConflict}
}
func validateTransactionState(state TransactionState) error {
 if state.Execution.ProviderName==""{return ErrReferenceConflict}
 switch normalizeTransactionKind(state.Kind){
 case TransactionKindPPOB:
  if state.Request.ReferenceID==""{return ErrReferenceConflict}
 case TransactionKindPayment:
  if state.Payment==nil||state.Payment.ReferenceID==""{return ErrReferenceConflict}
  if err:=payment.ValidateTransaction(*state.Payment);err!=nil{return err}
  if state.Execution.Result.ReferenceID!=""&&state.Execution.Result.ReferenceID!=state.Payment.ReferenceID{return ErrReferenceConflict}
 default:return ErrReferenceConflict
 }
 return nil
}
func samePaymentObservedResult(a,b *payment.Transaction) bool {
 if a==nil||b==nil{return false}
 return a.ReferenceID==b.ReferenceID&&a.ProviderReference==b.ProviderReference&&a.Amount==b.Amount&&a.Currency==b.Currency&&a.CustomerID==b.CustomerID&&a.Description==b.Description&&a.Status==b.Status&&a.Message==b.Message
}
func sameTransactionObservedResult(a,b TransactionState) bool {
 if normalizeTransactionKind(a.Kind)!=normalizeTransactionKind(b.Kind){return false}
 if normalizeTransactionKind(a.Kind)==TransactionKindPayment{return samePaymentObservedResult(a.Payment,b.Payment)}
 return samePurchaseResult(a.Execution.Result,b.Execution.Result)
}
func paymentProviderReference(state TransactionState) string {if state.Payment==nil{return ""};return state.Payment.ProviderReference}
func paymentCurrency(state TransactionState) string {if state.Payment==nil{return ""};return state.Payment.Currency}
func paymentCustomerID(state TransactionState) string {if state.Payment==nil{return ""};return state.Payment.CustomerID}
func paymentDescription(state TransactionState) string {if state.Payment==nil{return ""};return state.Payment.Description}
