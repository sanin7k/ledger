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

	prevChecksum := uint32(0)
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
	success := 1 // leader counts as one
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

		ok, _ := l.send(addr, req)
		if ok {
			success++
		}
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
