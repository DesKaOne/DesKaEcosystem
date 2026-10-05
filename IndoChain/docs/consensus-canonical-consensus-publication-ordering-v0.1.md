# IndoChain v0.1 — Canonical Commit ↔ Consensus Publication Ordering Contract

## Milestone 4.49

Milestone ini memperkuat regression boundary antara consensus runtime finality evidence dan canonical node/storage publication.

### Ordering contract

1. Consensus runtime dapat menghasilkan authenticated finality evidence untuk candidate block.
2. Finality evidence tidak dengan sendirinya memajukan canonical node head/state.
3. Node execution membangun candidate state dan menyerahkannya ke canonical storage commit boundary.
4. Canonical node Head, HeadHash, dan State hanya dipublish setelah CommitBlockState berhasil.
5. Jika canonical storage commit gagal, node canonical state dan persisted storage state tetap pada boundary sebelumnya.
6. Runtime finality evidence tetap tersedia sebagai evidence; milestone ini tidak menambahkan automatic retry/resubmission.

### Regression coverage

- TestCommitRuntimeFinalizedBlockStoreFailureDoesNotPublishCanonicalNodeState
  - menjalankan runtime sampai finality, memaksa canonical store failure, lalu memastikan node head/state dan storage head/state tidak maju.
  - memastikan finality certificate tetap tersedia sebagai evidence setelah commit failure.
- TestCommitRuntimeFinalizedBlockPublishesCanonicalStateOnlyAfterSuccessfulCommit
  - memastikan node tetap di genesis sebelum handoff canonical.
  - setelah successful commit, node head/hash/state harus sama dengan canonical storage.

### Production boundary

Tidak ada WAL writer/reader production, snapshot recovery activation, filesystem crash recovery, automatic retry/resubmission, atau perubahan pada BFT protocol. Perubahan milestone ini bersifat regression-test coverage terhadap boundary node yang sudah ada.

### Known limitations

- Test failure injection masih menggunakan test-only MemoryStore wrapper.
- Process crash, power loss, dan distributed recovery belum diuji.
- Runtime finality evidence generation tetap merupakan development consensus runtime dan bukan klaim production BFT completion.
- Durable WAL/snapshot recovery tetap di luar scope.

### Verification

Implementation commit: 75daf849b52466b30e1e78f15a8485f1b66fa288.

CI verification is required on the exact final documentation HEAD before milestone completion is declared.
