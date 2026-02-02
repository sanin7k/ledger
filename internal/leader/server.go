package leader

import (
	"encoding/binary"
	"errors"
	"log"
	"net"

	"github.com/sanin7k/ledger/internal/protocol"
)

func Serve(addr string, l *Leader) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	log.Printf("leader listening on %s", addr)

	go func() {
		for _, followerAddr := range l.followers {
			if err := l.catchUpFollower(followerAddr); err != nil {
				log.Printf("startup catch-up failed for %s: %v", followerAddr, err)
			}
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go l.handleClient(conn)
	}
}

func (l *Leader) handleClient(conn net.Conn) {
	defer conn.Close()

	// Client sends: [uint32 payloadLen][payload]
	lenBuf := make([]byte, 4)
	if _, err := conn.Read(lenBuf); err != nil {
		return
	}
	n := binary.BigEndian.Uint32(lenBuf)

	payload := make([]byte, n)
	if _, err := conn.Read(payload); err != nil {
		return
	}

	err := l.append(payload)
	if err != nil {
		conn.Write([]byte("ERROR"))
		return
	}

	conn.Write([]byte("OK"))
}

func (l *Leader) append(payload []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	newIndex := l.log.LastIndex() + 1

	// Compute checksum of previous entry
	var prevChecksum uint32 = 0
	if newIndex > 1 {
		prevEntry, err := l.log.Read(newIndex - 1)
		if err != nil {
			return err
		}
		prevChecksum = prevEntry.Checksum
	}

	// 1. Append locally (durable, uncommitted)
	if err := l.log.Append(newIndex, payload); err != nil {
		return err
	}

	// 2. Replicate to followers
	success := 1 // leader itself
	quorum := (len(l.followers)+1)/2 + 1

	for _, addr := range l.followers {
		req := protocol.AppendRequest{
			LeaderID:          l.id,
			PrevIndex:         newIndex - 1,
			PrevChecksum:      prevChecksum,
			Index:             newIndex,
			Payload:           payload,
			LeaderCommitIndex: l.log.CommitIndex(),
		}

		ok, _, reachable := l.send(addr, req)

		if !reachable {
			// follower down → ignore for this round
			continue
		}

		if !ok {
			// follower reachable but diverged → attempt catch-up
			_ = l.catchUpFollower(addr)
			continue
		}

		success++
	}

	if success < quorum {
		return errors.New("failed to reach quorum")
	}

	// 3. Commit (durable)
	if err := l.log.Commit(newIndex); err != nil {
		return err
	}

	return nil
}

func (l *Leader) catchUpFollower(addr string) error {
	matchIndex, err := l.probeFollowerPrefix(addr)
	if err != nil {
		return err
	}
	return l.replicateFromIndex(addr, matchIndex)
}

func (l *Leader) probeFollowerPrefix(addr string) (uint64, error) {
	prevIndex := l.log.LastIndex()

	for {
		var prevChecksum uint32 = 0
		if prevIndex > 0 {
			e, err := l.log.Read(prevIndex)
			if err != nil {
				return 0, err
			}
			prevChecksum = e.Checksum
		}

		req := protocol.AppendRequest{
			LeaderID:     l.id,
			PrevIndex:    prevIndex,
			PrevChecksum: prevChecksum,
			Payload:      nil, // PROBE
		}

		ok, lastIndex, reachable := l.send(addr, req)

		if !reachable {
			// follower is down → stop catch-up
			return 0, errors.New("follower unreachable")
		}

		if ok {
			return prevIndex, nil
		}

		// reachable but prefix mismatch
		prevIndex = lastIndex
	}
}

func (l *Leader) replicateFromIndex(addr string, start uint64) error {
	for i := start + 1; i <= l.log.LastIndex(); i++ {
		entry, err := l.log.Read(i)
		if err != nil {
			return err
		}

		var prevChecksum uint32 = 0
		if i > 1 {
			prev, _ := l.log.Read(i - 1)
			prevChecksum = prev.Checksum
		}

		req := protocol.AppendRequest{
			LeaderID:          l.id,
			PrevIndex:         i - 1,
			PrevChecksum:      prevChecksum,
			Index:             i,
			Payload:           entry.Payload,
			LeaderCommitIndex: l.log.CommitIndex(),
		}
		ok, _, reachable := l.send(addr, req)
		if !reachable {
			return errors.New("follower unreachable during replication")
		}
		if !ok {
			return errors.New("replication rejected during catch-up")
		}
	}
	return nil
}
