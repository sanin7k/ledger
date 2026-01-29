package log

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func newTestLog(t *testing.T) (*Log, string) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	return l, dir
}

func reopenLog(t *testing.T, dir string) *Log {
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	return l
}

func TestCrashBeforePayloadWrite(t *testing.T) {
	l, dir := newTestLog(t)

	// Simulate crash immediately
	l.dataFile.Close()

	l2 := reopenLog(t, dir)

	if l2.LastIndex() != 0 {
		t.Fatalf("expected empty log, got %d", l2.LastIndex())
	}
}

func TestCrashAfterPayloadBeforeMarker(t *testing.T) {
	l, dir := newTestLog(t)

	// Manually write partial entry
	index := uint64(1)
	payload := []byte("hello")

	buf := make([]byte, 16+len(payload))
	binary.BigEndian.PutUint64(buf[0:8], index)
	binary.BigEndian.PutUint32(buf[8:12], uint32(len(payload)))
	copy(buf[16:], payload)

	if _, err := l.dataFile.Write(buf); err != nil {
		t.Fatal(err)
	}
	l.dataFile.Sync()

	// Simulate crash before marker
	l.dataFile.Close()

	l2 := reopenLog(t, dir)

	if l2.LastIndex() != 0 {
		t.Fatalf("expected entry to be discarded, got %d", l2.LastIndex())
	}
}

func TestCrashAfterMarkerBeforeCommit(t *testing.T) {
	l, dir := newTestLog(t)

	err := l.Append(1, []byte("A"))
	if err != nil {
		t.Fatal(err)
	}

	// Crash before commit index update
	l.dataFile.Close()
	l.metaFile.Close()

	l2 := reopenLog(t, dir)

	if l2.LastIndex() != 1 {
		t.Fatalf("expected entry to survive, got %d", l2.LastIndex())
	}
	if l2.CommitIndex() != 0 {
		t.Fatalf("entry must not be committed")
	}
}

func TestCrashDuringCommitIndexUpdate(t *testing.T) {
	l, dir := newTestLog(t)

	if err := l.Append(1, []byte("X")); err != nil {
		t.Fatal(err)
	}

	// Simulate crash by writing tmp but not renaming
	tmpPath := filepath.Join(dir, metaTmpFile)
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	tmp.Write([]byte{0, 0, 0, 0, 0, 0, 0, 1})
	tmp.Sync()
	tmp.Close()

	// No rename performed — crash here

	l2 := reopenLog(t, dir)

	if l2.CommitIndex() != 0 {
		t.Fatalf("commit index must not advance on partial update")
	}
}

func TestCommitPersists(t *testing.T) {
	l, dir := newTestLog(t)

	if err := l.Append(1, []byte("Z")); err != nil {
		t.Fatal(err)
	}
	if err := l.Commit(1); err != nil {
		t.Fatal(err)
	}

	l.dataFile.Close()
	l.metaFile.Close()

	l2 := reopenLog(t, dir)

	if l2.CommitIndex() != 1 {
		t.Fatalf("commit index lost after restart")
	}
}

func TestCommitIndexBeyondLogFails(t *testing.T) {
	dir := t.TempDir()

	// Write invalid meta
	meta := filepath.Join(dir, "log.meta")
	os.WriteFile(meta, []byte{
		0, 0, 0, 0, 0, 0, 0, 5,
	}, 0644)

	_, err := Open(dir)
	if err == nil {
		t.Fatalf("expected failure on invalid commit index")
	}
}

func TestReadEntry(t *testing.T) {
	l, _ := newTestLog(t)

	if err := l.Append(1, []byte("A")); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(2, []byte("B")); err != nil {
		t.Fatal(err)
	}

	e, err := l.Read(2)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if e.Index != 2 || string(e.Payload) != "B" {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestReadNonExistentEntry(t *testing.T) {
	l, _ := newTestLog(t)

	_, err := l.Read(1)
	if err == nil {
		t.Fatalf("expected error for non-existent entry")
	}
}

func TestTruncateSpeculativeEntries(t *testing.T) {
	l, _ := newTestLog(t)

	l.Append(1, []byte("A"))
	l.Append(2, []byte("B"))
	l.Append(3, []byte("C"))

	if err := l.TruncateFrom(2); err != nil {
		t.Fatal(err)
	}

	if l.LastIndex() != 1 {
		t.Fatalf("expected lastIndex=1, got %d", l.LastIndex())
	}

	if _, err := l.Read(2); err == nil {
		t.Fatalf("expected entry 2 to be truncated")
	}
}

func TestTruncateCommittedFails(t *testing.T) {
	l, _ := newTestLog(t)

	l.Append(1, []byte("A"))
	l.Commit(1)

	if err := l.TruncateFrom(1); err == nil {
		t.Fatalf("expected failure truncating committed entry")
	}
}

func TestTruncatePersistsAcrossRestart(t *testing.T) {
	l, dir := newTestLog(t)

	l.Append(1, []byte("A"))
	l.Append(2, []byte("B"))
	l.TruncateFrom(2)

	l.Close()

	l2 := reopenLog(t, dir)

	if l2.LastIndex() != 1 {
		t.Fatalf("expected lastIndex=1 after restart")
	}
}
