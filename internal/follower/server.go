package follower

import (
	"log"
	"net"

	"github.com/sanin7k/ledger/internal/protocol"
	"github.com/sanin7k/ledger/internal/transport"
)

func handleConn(conn net.Conn, f *Follower) {
	defer conn.Close()

	frame, err := transport.ReadFrame(conn)
	if err != nil {
		return
	}

	if frame.Type != protocol.MsgAppendRequest {
		return
	}

	req, err := protocol.DecodeAppendRequest(frame.Payload)
	if err != nil {
		return
	}

	resp := f.HandleAppend(req)

	payload, err := protocol.EncodeAppendResponse(resp)
	if err != nil {
		return
	}

	respFrame := transport.Frame{
		Type:    protocol.MsgAppendResponse,
		Payload: payload,
	}

	_ = transport.WriteFrame(conn, respFrame)
}

func Serve(addr string, f *Follower) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	log.Printf("follower listening on %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}

		// v1: handle synchronously
		handleConn(conn, f)
	}
}
