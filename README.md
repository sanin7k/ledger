# ledger

Replicated write-ahead log with crash recovery.

---

## Overview

`ledger` is an experimental systems project implementing a replicated
append-only log under a single-leader model.

The focus is on durability, crash recovery, and correctness of replication
state under crash-stop failures.

---

## System Model

- Fixed-size cluster (3–5 nodes)
- Single leader (assumed; no leader election)
- Clients send append requests only to the leader
- Followers act as passive replicas
- Communication over raw TCP
- Nodes may crash and restart with disk intact
- Network may drop, delay, or reorder messages

---

## Failure Model

### Handled

- Single-node crashes
- Crashes at any instruction boundary
- Restart with persistent storage intact
- Message loss, duplication, and reordering

### Not Handled

- Byzantine behavior
- Disk corruption
- Concurrent leaders
- Dynamic membership

---

## Log Structure

- Indexed append-only log
- Entries become durable after a completion marker is written
- Incomplete trailing entries are removed during recovery
- Entries at or below `commit_index` are immutable

---

## Commitment Model

- Commitment is represented by a durable `commit_index`
- Commitment decisions are made by the leader
- Committed entries are never removed or overwritten
- Uncommitted entries may be truncated or replaced

---

## Replication Model

### Leader

- Appends entries
- Replicates entries to followers
- Advances `commit_index` after majority durability
- Does not advance `commit_index` during recovery

### Follower

- Maintains a prefix of the leader’s log
- Accepts entries in order only
- Truncates entries above `commit_index` when required
- Does not decide commitment independently

---

## Crash Recovery

- Recovery uses only durable on-disk state
- Logs are repaired before replication resumes
- Entries above `commit_index` are treated as uncommitted

---

## Non-Goals

- High availability under minority failure
- Reads during leader downtime
- Leader election
- Dynamic membership
- Byzantine fault tolerance

---

## Documentation

- Safety and correctness invariants: `docs/invariants.md`

---

## Status

Work in progress (v1).

---

## License

MIT
