package leader

import (
	"net"
	"sync"
	"time"

	"github.com/sanin7k/ledger/internal/log"
	"github.com/sanin7k/ledger/internal/protocol"
	"github.com/sanin7k/ledger/internal/transport"
)

type appendFunc func(addr string, req protocol.AppendRequest) (bool, uint64, bool)

type Leader struct {
	id        uint32
	log       *log.Log
	followers []string // follower TCP addresses

	send appendFunc
	mu   sync.Mutex // single append at a time
}

func NewLeader(id uint32, log *log.Log, followers []string) *Leader {
	return &Leader{
		id:        id,
		log:       log,
		followers: followers,
		send:      sendAppend,
	}
}

func sendAppend(addr string, req protocol.AppendRequest) (bool, uint64, bool) {
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false, 0, false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(200 * time.Millisecond))

	payload, err := protocol.EncodeAppendRequest(req)
	if err != nil {
		return false, 0, false
	}

	err = transport.WriteFrame(conn, transport.Frame{
		Type:    protocol.MsgAppendRequest,
		Payload: payload,
	})
	if err != nil {
		return false, 0, false
	}

	frame, err := transport.ReadFrame(conn)
	if err != nil {
		return false, 0, false
	}

	if frame.Type != protocol.MsgAppendResponse {
		return false, 0, false
	}

	resp, err := protocol.DecodeAppendResponse(frame.Payload)
	if err != nil {
		return false, 0, false
	}

	return resp.Success, resp.LastIndex, true
}
