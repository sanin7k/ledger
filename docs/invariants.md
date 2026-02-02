# ledger v1 — Safety & Correctness Invariants

This document defines the safety and correctness invariants for `ledger` v1.
All implementation behavior **must** comply with these invariants.
If code behavior contradicts this document, the code is incorrect.

---

## 1. System Model

- Fixed-size cluster of N nodes (3–5)
- Single leader (assumed; no leader election in v1)
- Clients send append requests only to the leader
- Followers are passive replicas
- Communication via raw TCP
- Nodes may crash and restart with persistent storage intact
- Network may drop, delay, duplicate, or reorder messages

---

## 2. Failure Model

### Tolerated

- Crash-stop failure of any single node
- Crashes at any instruction boundary
- Restart with durable on-disk state intact
- Partial writes and torn log entries
- Message loss, duplication, and reordering
- Unreachable or slow followers

### Not Tolerated

- Byzantine behavior
- Disk corruption
- Concurrent leaders
- Dynamic membership changes

---

## 3. Log Structure Invariants

### Invariant L1 — Indexed Append-Only Log

- Each log entry has a monotonically increasing index
- Entries are written only by appending
- Entries with index ≤ `commit_index` are immutable

---

### Invariant L2 — Durability Marker

- An entry is considered durable **iff** a completion marker is present
- Entries without a completion marker are invalid
- On restart, all invalid trailing entries **must** be truncated

---

### Invariant L3 — Prefix Property

- At all times, a follower’s log is a prefix of the leader’s log
- A follower must never contain entries that diverge from the leader’s committed history

---

## 4. Commitment Invariants

### Invariant C1 — Commit Index Authority

- `commit_index` is the sole authoritative indicator of commitment
- `commit_index` is stored durably on disk
- Commitment is never inferred from volatile state or network communication after a crash

---

### Invariant C2 — Committed Entries Are Immutable

Any entry with index ≤ `commit_index`:

- must never be removed
- must never be overwritten
- must never be reordered

---

### Invariant C3 — Uncommitted Entries Are Speculative

Entries with index > `commit_index`:

- may be truncated
- may be overwritten
- carry no client-visible guarantees

---

## 5. Client Safety Invariants

### Invariant S1 — No Lies to the Client

A client may receive a success response **only if** the appended entry:

- is durably written on the leader
- is durably written on a majority of nodes
- has been reflected in a durable `commit_index`

---

### Invariant S2 — Success Implies Permanence

If a client receives success for entry *i*, then:

- entry *i* will survive any single-node crash
- entry *i* will appear in all future system histories

---

## 6. Leader Invariants

### Invariant LDR1 — Single Source of Truth

- The leader’s log defines the authoritative history
- Followers never invent, reorder, or finalize history

---

### Invariant LDR2 — Commitment Is a Durable Act

The leader advances `commit_index` **only after**:

- the entry is durably replicated to a majority of nodes

The leader must never advance `commit_index` during crash recovery.

---

### Invariant LDR3 — Recovery Does Not Decide Commitment

After a crash, the leader:

- repairs its log
- reads `commit_index` from durable storage
- treats all entries with index > `commit_index` as uncommitted

Commitment decisions occur **only** during normal operation.

---

## 7. Follower Invariants

### Invariant FLW1 — Prefix Enforcement

- A follower must reject or truncate any entry that violates the prefix property
- A follower must never accept entries out of order

---

### Invariant FLW2 — No Independent Commitment

- A follower never decides that an entry is committed
- A follower persists `commit_index` values only as instructed by the leader

---

### Invariant FLW3 — Safe Truncation

- A follower may truncate entries with index > `commit_index`
- A follower must never truncate entries with index ≤ `commit_index`

---

## 8. Crash Recovery Invariants

### Invariant R1 — Deterministic Recovery

- After restart, recovery decisions depend only on durable on-disk state
- No recovery logic relies on network communication to infer commitment

---

### Invariant R2 — Log Repair First

On restart, a node must:

- scan from the end of the log
- truncate incomplete entries
- restore the log to a fully durable state

Only after repair may replication resume.

---

## 9. Progress and Availability (Non-Goals)

- The system may block if a majority is unavailable
- Availability is sacrificed in favor of safety
- No guarantees are made for reads during leader downtime
- Followers may remain stale until explicitly repaired by the leader

---

## 10. Design Philosophy

- Safety > availability
- Correctness > performance
- Durable facts > inferred state
- Blocking is acceptable; lying is not

---

End of invariants.

