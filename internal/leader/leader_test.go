package leader

import (
	"testing"

	"github.com/sanin7k/ledger/internal/log"
	"github.com/sanin7k/ledger/internal/protocol"
)

func newTestLeader(t *testing.T, followerCount int) (*Leader, *log.Log) {
	dir := t.TempDir()

	lg, err := log.Open(dir)
	if err != nil {
		t.Fatalf("log open failed: %v", err)
	}

	followers := make([]string, followerCount)
	for i := range followers {
		followers[i] = "follower"
	}

	ldr := NewLeader(1, lg, followers)
	return ldr, lg
}

func TestLeaderAppendSuccess(t *testing.T) {
	ldr, lg := newTestLeader(t, 2) // leader + 2 followers = 3 nodes

	acks := 0
	ldr.send = func(addr string, req protocol.AppendRequest) (bool, uint64) {
		acks++
		return true, req.Index
	}

	for i := 0; i < 3; i++ {
		err := ldr.append([]byte("hello"))
		if err != nil {
			t.Fatalf("append failed: %v", err)
		}
	}

	if lg.LastIndex() != 3 {
		t.Fatalf("expected lastIndex=3, got %d", lg.LastIndex())
	}
	if lg.CommitIndex() != 3 {
		t.Fatalf("expected commitIndex=3, got %d", lg.CommitIndex())
	}
}

func TestLeaderAppendFailsWithoutQuorum(t *testing.T) {
	ldr, lg := newTestLeader(t, 2)

	ldr.send = func(addr string, req protocol.AppendRequest) (bool, uint64) {
		return false, 0 // followers reject
	}

	err := ldr.append([]byte("X"))
	if err == nil {
		t.Fatalf("expected failure without quorum")
	}

	if lg.CommitIndex() != 0 {
		t.Fatalf("commitIndex must not advance")
	}
}

func TestLeaderAppendPartialQuorum(t *testing.T) {
	ldr, lg := newTestLeader(t, 2)

	calls := 0
	ldr.send = func(addr string, req protocol.AppendRequest) (bool, uint64) {
		calls++
		if calls == 1 {
			return true, req.Index
		}
		return false, 0
	}

	err := ldr.append([]byte("A"))
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	if lg.CommitIndex() != 1 {
		t.Fatalf("expected commitIndex=1")
	}
}

func TestLeaderCountsItself(t *testing.T) {
	ldr, lg := newTestLeader(t, 1) // leader + 1 follower

	ldr.send = func(addr string, req protocol.AppendRequest) (bool, uint64) {
		return true, req.Index
	}

	err := ldr.append([]byte("A"))
	if err != nil {
		t.Fatalf("append failed")
	}

	if lg.CommitIndex() != 1 {
		t.Fatalf("commitIndex not advanced")
	}
}
