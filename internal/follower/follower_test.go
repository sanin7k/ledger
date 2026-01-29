package follower

import (
	"hash/crc32"
	"testing"

	"github.com/sanin7k/ledger/internal/log"
	"github.com/sanin7k/ledger/internal/protocol"
)

func newFollower(t *testing.T) (*Follower, *log.Log, string) {
	dir := t.TempDir()
	l, err := log.Open(dir)
	if err != nil {
		t.Fatalf("log open failed: %v", err)
	}
	f := NewFollower(1, l)
	return f, l, dir
}

func reopenFollower(t *testing.T, dir string) (*Follower, *log.Log) {
	l, err := log.Open(dir)
	if err != nil {
		t.Fatalf("log reopen failed: %v", err)
	}
	f := NewFollower(1, l)
	return f, l
}

func TestFollowerAppendSuccess(t *testing.T) {
	f, l, _ := newFollower(t)

	resp := f.HandleAppend(protocol.AppendRequest{
		PrevIndex:         0,
		PrevChecksum:      0,
		Index:             1,
		Payload:           []byte("A"),
		LeaderCommitIndex: 0,
	})

	if !resp.Success {
		t.Fatalf("expected success")
	}
	if l.LastIndex() != 1 {
		t.Fatalf("expected lastIndex=1, got %d", l.LastIndex())
	}
}

func TestFollowerRejectsMissingPrefix(t *testing.T) {
	f, _, _ := newFollower(t)

	resp := f.HandleAppend(protocol.AppendRequest{
		PrevIndex:         1,
		PrevChecksum:      123,
		Index:             2,
		Payload:           []byte("B"),
		LeaderCommitIndex: 0,
	})

	if resp.Success {
		t.Fatalf("expected failure due to missing prefix")
	}
}

func TestFollowerTruncatesOnPrefixMismatch(t *testing.T) {
	f, l, _ := newFollower(t)

	// Append entry 1
	f.HandleAppend(protocol.AppendRequest{
		PrevIndex: 0,
		Index:     1,
		Payload:   []byte("A"),
	})

	// Append entry 2
	f.HandleAppend(protocol.AppendRequest{
		PrevIndex:    1,
		PrevChecksum: crc32.ChecksumIEEE([]byte("A")), // assume correct
		Index:        2,
		Payload:      []byte("B"),
	})

	// Leader sends conflicting append at index 2
	resp := f.HandleAppend(protocol.AppendRequest{
		PrevIndex:    1,
		PrevChecksum: 9999, // wrong checksum
		Index:        2,
		Payload:      []byte("C"),
	})

	// Must fail
	if resp.Success {
		t.Fatalf("expected failure after prefix mismatch")
	}

	// But truncation must have happened
	if l.LastIndex() != 0 {
		t.Fatalf("expected truncation to empty log, got %d", l.LastIndex())
	}
}

func TestFollowerRejectsOverwriteCommitted(t *testing.T) {
	f, l, _ := newFollower(t)

	f.HandleAppend(protocol.AppendRequest{
		PrevIndex: 0,
		Index:     1,
		Payload:   []byte("A"),
	})

	if err := l.Commit(1); err != nil {
		t.Fatal(err)
	}

	resp := f.HandleAppend(protocol.AppendRequest{
		PrevIndex: 0,
		Index:     1,
		Payload:   []byte("B"),
	})

	if resp.Success {
		t.Fatalf("expected rejection of overwrite on committed entry")
	}
}

func TestFollowerCommitPropagation(t *testing.T) {
	f, l, _ := newFollower(t)

	f.HandleAppend(protocol.AppendRequest{
		PrevIndex:         0,
		Index:             1,
		Payload:           []byte("A"),
		LeaderCommitIndex: 1,
	})

	if l.CommitIndex() != 1 {
		t.Fatalf("expected commitIndex=1, got %d", l.CommitIndex())
	}
}

func TestFollowerCrashRecovery(t *testing.T) {
	f, l, dir := newFollower(t)

	f.HandleAppend(protocol.AppendRequest{
		PrevIndex: 0,
		Index:     1,
		Payload:   []byte("A"),
	})

	if err := l.Commit(1); err != nil {
		t.Fatal(err)
	}

	// Simulate crash
	l.Close()

	f2, l2 := reopenFollower(t, dir)

	if l2.LastIndex() != 1 {
		t.Fatalf("expected lastIndex=1 after restart")
	}
	if l2.CommitIndex() != 1 {
		t.Fatalf("expected commitIndex=1 after restart")
	}

	// Try illegal overwrite
	resp := f2.HandleAppend(protocol.AppendRequest{
		PrevIndex: 0,
		Index:     1,
		Payload:   []byte("B"),
	})

	if resp.Success {
		t.Fatalf("expected overwrite rejection after restart")
	}
}
