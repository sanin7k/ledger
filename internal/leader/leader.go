package leader

import (
	"net"
	"sync"

	"github.com/sanin7k/ledger/internal/log"
	"github.com/sanin7k/ledger/internal/protocol"
	"github.com/sanin7k/ledger/internal/transport"
)

type appendFunc func(addr string, req protocol.AppendRequest) (bool, uint64)

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

func sendAppend(addr string, req protocol.AppendRequest) (bool, uint64) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false, 0
	}
	defer conn.Close()

	payload, err := protocol.EncodeAppendRequest(req)
	if err != nil {
		return false, 0
	}

	err = transport.WriteFrame(conn, transport.Frame{
		Type:    protocol.MsgAppendRequest,
		Payload: payload,
	})
	if err != nil {
		return false, 0
	}

	frame, err := transport.ReadFrame(conn)
	if err != nil {
		return false, 0
	}

	if frame.Type != protocol.MsgAppendResponse {
		return false, 0
	}

	resp, err := protocol.DecodeAppendResponse(frame.Payload)
	if err != nil {
		return false, 0
	}

	return resp.Success, resp.LastIndex
}
