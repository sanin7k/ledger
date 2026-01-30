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

func startFollower(t *testing.T, id uint32, addr string, dir string) {
	lg, err := log.Open(dir)
	if err != nil {
		t.Fatalf("follower log open failed: %v", err)
	}

	f := follower.NewFollower(id, lg)

	go func() {
		err := follower.Serve(addr, f)
		if err != nil {
			t.Errorf("follower serve failed: %v", err)
		}
	}()

	// Give TCP listener time to start
	time.Sleep(50 * time.Millisecond)
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

func TestLeaderFollowerEndToEnd(t *testing.T) {
	baseDir := t.TempDir()

	// --- Start followers ---
	f1Dir := filepath.Join(baseDir, "follower1")
	f2Dir := filepath.Join(baseDir, "follower2")
	os.MkdirAll(f1Dir, 0755)
	os.MkdirAll(f2Dir, 0755)

	startFollower(t, 1, "127.0.0.1:9101", f1Dir)
	startFollower(t, 2, "127.0.0.1:9102", f2Dir)

	// --- Start leader ---
	leaderDir := filepath.Join(baseDir, "leader")
	os.MkdirAll(leaderDir, 0755)

	lg, err := log.Open(leaderDir)
	if err != nil {
		t.Fatalf("leader log open failed: %v", err)
	}

	ldr := leader.NewLeader(
		100,
		lg,
		[]string{
			"127.0.0.1:9101",
			"127.0.0.1:9102",
		},
	)

	go func() {
		err := leader.Serve("127.0.0.1:9100", ldr)
		if err != nil {
			t.Errorf("leader serve failed: %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// --- Client append ---
	resp := sendClientAppend(t, "127.0.0.1:9100", []byte("hello-ledger"))

	if string(resp) != "OK" {
		t.Fatalf("expected OK, got %q", resp)
	}

	// --- Verify durability ---
	lg2, _ := log.Open(leaderDir)
	if lg2.CommitIndex() != 1 {
		t.Fatalf("leader commitIndex = %d, expected 1", lg2.CommitIndex())
	}

	f1Log, _ := log.Open(f1Dir)
	f2Log, _ := log.Open(f2Dir)

	if f1Log.LastIndex() != 1 || f2Log.LastIndex() != 1 {
		t.Fatalf("followers did not replicate entry")
	}
}
