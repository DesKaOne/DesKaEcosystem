# IndoChain v0.1 — Finalized Block Replay & Stale Context Contract

## Milestone 4.50

Milestone ini memperluas regression boundary pada node canonical handoff untuk finalized block replay dan stale consensus context.

### Locked invariants

1. Exact replay atas finalized block yang sudah committed ditolak dengan ErrFinalizedBlockAlreadyCommitted.
2. Block berbeda pada canonical height yang sudah committed tidak boleh bypass canonical context validation.
3. Stale consensus context ditolak sebelum storage mutation.
4. Replay/stale rejection tidak mengubah node Head/HeadHash/State maupun canonical storage.
5. Tidak ada automatic retry/resubmission atau perubahan protocol BFT.

### Regression coverage

- TestCommitFinalizedBlockRejectsAlreadyCommittedBlockWithoutMutation
  - existing exact replay regression.
- TestCommitFinalizedBlockRejectsDifferentBlockAtCommittedHeightWithoutMutation
  - candidate berbeda pada height yang sudah committed harus berhenti di canonical context boundary dan tidak mengubah node.
- TestCommitFinalizedBlockRejectsStaleContextWithoutStorageMutation
  - context height/previous-hash stale harus ditolak tanpa mutation pada node maupun storage.

### Production boundary

Milestone ini hanya menambah test coverage terhadap CommitFinalizedBlock yang sudah ada. Tidak ada production WAL/snapshot activation, retry/resubmission, new canonical state mutation, atau production BFT claim.

### Known limitations

- Test menggunakan MemoryStore dan fixture finalized block yang sudah ada.
- Tidak menguji distributed replay race atau process crash recovery.
- Durable WAL/snapshot recovery tetap di luar scope.

### Verification

Implementation commit: 5d4a9518eb9aac7a55bb394ed9734a1fdcd415fe.

Exact-head CI gate pending.
