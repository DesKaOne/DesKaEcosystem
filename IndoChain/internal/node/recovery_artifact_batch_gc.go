package node

import (
	"errors"
	"sort"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

// ReconcileCandidateRecoveryArtifacts enumerates every durable candidate
// identity in a deterministic order and applies the node-owned per-artifact
// classification/cleanup policy. Canonical state is never mutated by this pass.
func (n *Node) ReconcileCandidateRecoveryArtifacts(store storage.CandidateStore) ([]RecoveryArtifactReport, error) {
	if n == nil || n.Store == nil || n.State == nil {
		return nil, ErrNilStore
	}
	enumerator, ok := store.(storage.CandidateEnumerator)
	if !ok {
		return nil, errors.New("candidate store does not support deterministic enumeration")
	}
	keys, err := enumerator.ListCandidateKeys()
	if err != nil {
		return nil, err
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Height != keys[j].Height {
			return keys[i].Height < keys[j].Height
		}
		return string(keys[i].Hash[:]) < string(keys[j].Hash[:])
	})

	reports := make([]RecoveryArtifactReport, 0, len(keys))
	for _, key := range keys {
		report, err := n.ReconcileCandidateRecoveryArtifact(store, key)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
