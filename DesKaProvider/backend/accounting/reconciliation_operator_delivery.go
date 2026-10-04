package accounting

import (
	"encoding/json"
	"errors"
	"io"
)

// EncodeReconciliationOperatorReportV1 writes the versioned operator report
// contract to w as JSON. It is strictly read-only: encoding never resolves
// evidence, writes persistence, calls a provider, or changes financial state.
func EncodeReconciliationOperatorReportV1(w io.Writer, report ReconciliationReport) error {
	if w == nil {
		return errors.New("reconciliation operator report writer is required")
	}
	return json.NewEncoder(w).Encode(report.OperatorReport().V1())
}
