# ledger

Replicated write-ahead log with crash recovery.

---

## Overview

`ledger` is an experimental systems project implementing a replicated,
append-only write-ahead log under a single-leader model.

The project focuses on **durability**, **crash recovery**, and **explicit
correctness guarantees** under crash-stop failures and unreliable networks.

The goal is not performance or feature completeness, but a clear and
auditable replication core with well-defined invariants.

---

## System Model

- Fixed-size cluster (3–5 nodes)
- Single leader (assumed; no leader election in v1)
- Clients send append requests only to the leader
- Followers act as passive replicas
- Communication over raw TCP (no HTTP, no frameworks)
- Nodes may crash and restart with persistent storage intact
- Network may drop, delay, duplicate, or reorder messages

---

## Failure Model

### Handled

- Crash-stop failures
- Crashes at any instruction boundary
- Restart with durable state intact
- Partial writes and torn log entries
- Message loss, duplication, and reordering
- Unreachable or slow followers

### Not Handled

- Byzantine behavior
- Disk corruption
- Concurrent leaders
- Dynamic membership
- Network partitions with multiple leaders

---

## Log Structure

- Indexed, append-only log
- Entries become durable only after a completion marker is written
- Incomplete trailing entries are removed during recovery
- Entries at or below `commit_index` are immutable
- Entries above `commit_index` are treated as uncommitted

---

## Commitment Model

- Commitment is represented by a durable `commit_index`
- Commitment decisions are made exclusively by the leader
- An entry is committed only after durable replication to a quorum
- Committed entries are never removed or overwritten
- Uncommitted entries may be truncated or replaced during recovery or catch-up

---

## Replication Model

### Leader

- Appends entries locally before replication
- Replicates entries to followers
- Advances `commit_index` only after quorum durability
- Does not advance `commit_index` during crash recovery
- Does not block on unreachable followers

### Follower

- Maintains a prefix of the leader’s log
- Accepts entries strictly in order
- Validates prefix using index and checksum
- Truncates divergent uncommitted entries when required
- Does not decide commitment independently

---

## Catch-Up Semantics

- Follower catch-up is **leader-driven**
- Catch-up is triggered when:
  - the leader starts (best-effort)
  - a reachable follower rejects an append
- Unreachable followers are skipped and do not block progress
- No background reconciliation or periodic heartbeats in v1

This design intentionally favors correctness and simplicity over eager convergence.

---

## Crash Recovery

- Recovery uses only durable on-disk state
- Partial entries are removed before replication resumes
- `commit_index` is validated against the log
- Entries above `commit_index` remain uncommitted after restart

---

## Non-Goals (v1)

- Leader election
- Dynamic membership
- High availability under minority failure
- Linearizable reads
- Background reconciliation or heartbeats
- Performance optimizations (batching, pipelining, async fan-out)
- Byzantine fault tolerance

---

## Documentation

- Safety and correctness invariants: `docs/invariants.md`

---

## Status

**v1 complete.**  
The system implements a correct, crash-safe replicated log core with
explicit invariants and bounded failure behavior.

Future versions may extend the system with leader election, heartbeats,
or performance optimizations without weakening v1 guarantees.

---

## License

MIT

