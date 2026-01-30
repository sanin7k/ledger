package integration

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sanin7k/ledger/internal/follower"
	"github.com/sanin7k/ledger/internal/leader"
	"github.com/sanin7k/ledger/internal/log"
)

func startFollower(t *testing.T, id uint32, addr string, dir string) func() {
	lg, err := log.Open(dir)
	if err != nil {
		t.Fatalf("follower log open failed: %v", err)
	}

	f := follower.NewFollower(id, lg)

	done := make(chan struct{})

	go func() {
		_ = follower.Serve(addr, f)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	return func() {
		lg.Close()
	}
}

func sendClientAppend(t *testing.T, addr string, payload []byte) []byte {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("client dial failed: %v", err)
	}
	defer conn.Close()

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(len(payload)))
	buf.Write(payload)

	if _, err := conn.Write(buf.Bytes()); err != nil {
		t.Fatalf("client write failed: %v", err)
	}

	resp := make([]byte, 16)
	n, err := conn.Read(resp)
	if err != nil {
		t.Fatalf("client read failed: %v", err)
	}

	return resp[:n]
}

func TestEndToEndHappyPath(t *testing.T) {
	baseDir := t.TempDir()

	f1Dir := filepath.Join(baseDir, "f1")
	f2Dir := filepath.Join(baseDir, "f2")
	os.MkdirAll(f1Dir, 0755)
	os.MkdirAll(f2Dir, 0755)

	startFollower(t, 1, "127.0.0.1:9201", f1Dir)
	startFollower(t, 2, "127.0.0.1:9202", f2Dir)

	leaderDir := filepath.Join(baseDir, "leader")
	os.MkdirAll(leaderDir, 0755)

	lg, _ := log.Open(leaderDir)
	ldr := leader.NewLeader(
		100,
		lg,
		[]string{
			"127.0.0.1:9201",
			"127.0.0.1:9202",
		},
	)

	go leader.Serve("127.0.0.1:9200", ldr)
	time.Sleep(50 * time.Millisecond)

	resp := sendClientAppend(t, "127.0.0.1:9200", []byte("ok"))

	if string(resp) != "OK" {
		t.Fatalf("expected OK, got %q", resp)
	}
}

func TestAppendWithOneFollowerDown(t *testing.T) {
	baseDir := t.TempDir()

	f1Dir := filepath.Join(baseDir, "f1")
	f2Dir := filepath.Join(baseDir, "f2")
	os.MkdirAll(f1Dir, 0755)
	os.MkdirAll(f2Dir, 0755)

	// Start only ONE follower
	startFollower(t, 1, "127.0.0.1:9301", f1Dir)

	leaderDir := filepath.Join(baseDir, "leader")
	os.MkdirAll(leaderDir, 0755)

	lg, _ := log.Open(leaderDir)
	ldr := leader.NewLeader(
		1,
		lg,
		[]string{
			"127.0.0.1:9301",
			"127.0.0.1:9302", // dead follower
		},
	)

	go leader.Serve("127.0.0.1:9300", ldr)
	time.Sleep(50 * time.Millisecond)

	resp := sendClientAppend(t, "127.0.0.1:9300", []byte("majority"))

	if string(resp) != "OK" {
		t.Fatalf("expected OK, got %q", resp)
	}

	lg2, _ := log.Open(leaderDir)
	if lg2.CommitIndex() != 1 {
		t.Fatalf("expected commitIndex=1, got %d", lg2.CommitIndex())
	}
}

func TestAppendWithoutQuorumFails(t *testing.T) {
	baseDir := t.TempDir()

	leaderDir := filepath.Join(baseDir, "leader")
	os.MkdirAll(leaderDir, 0755)

	lg, _ := log.Open(leaderDir)
	ldr := leader.NewLeader(
		1,
		lg,
		[]string{
			"127.0.0.1:9401",
			"127.0.0.1:9402",
		},
	)

	go leader.Serve("127.0.0.1:9400", ldr)
	time.Sleep(50 * time.Millisecond)

	resp := sendClientAppend(t, "127.0.0.1:9400", []byte("fail"))

	if string(resp) == "OK" {
		t.Fatalf("expected failure without quorum")
	}

	lg2, _ := log.Open(leaderDir)
	if lg2.CommitIndex() != 0 {
		t.Fatalf("commitIndex must not advance")
	}
}

func TestFollowerRestartAfterMissedAppend(t *testing.T) {
	baseDir := t.TempDir()

	f1Dir := filepath.Join(baseDir, "f1")
	os.MkdirAll(f1Dir, 0755)

	stopFollower := startFollower(t, 1, "127.0.0.1:9501", f1Dir)

	leaderDir := filepath.Join(baseDir, "leader")
	os.MkdirAll(leaderDir, 0755)

	lg, _ := log.Open(leaderDir)
	ldr := leader.NewLeader(
		1,
		lg,
		[]string{
			"127.0.0.1:9501",
		},
	)

	go leader.Serve("127.0.0.1:9500", ldr)
	time.Sleep(50 * time.Millisecond)

	// Kill follower before append
	stopFollower()

	resp := sendClientAppend(t, "127.0.0.1:9500", []byte("lost"))

	if string(resp) == "OK" {
		t.Fatalf("should not succeed without quorum")
	}

	// Restart follower
	startFollower(t, 1, "127.0.0.1:9501", f1Dir)

	// No corruption should exist
	fLog, _ := log.Open(f1Dir)
	if fLog.LastIndex() != 0 {
		t.Fatalf("follower log should be empty after restart")
	}
}
