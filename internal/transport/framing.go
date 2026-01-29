package transport

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
)

const (
	MaxFrameSize uint32 = 16 * 1024 * 1024 // 16 MB hard limit
)

type Frame struct {
	Type    uint8
	Payload []byte
}

func WriteFrame(conn net.Conn, frame Frame) error {
	payloadLen := uint32(1 + len(frame.Payload)) // 1 byte for Type

	if payloadLen > MaxFrameSize {
		return errors.New("frame too large")
	}

	// Allocate once, write once
	buf := make([]byte, 4+payloadLen)

	// Length prefix (does NOT include the length field itself)
	binary.BigEndian.PutUint32(buf[0:4], payloadLen)

	// Message type
	buf[4] = frame.Type

	// Payload
	copy(buf[5:], frame.Payload)

	// Write fully (handle short writes)
	total := 0
	for total < len(buf) {
		n, err := conn.Write(buf[total:])
		if err != nil {
			return err
		}
		total += n
	}

	return nil
}

func ReadFrame(conn net.Conn) (Frame, error) {
	var frame Frame

	// Read length prefix
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return frame, err
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 || length > MaxFrameSize {
		return frame, errors.New("invalid frame length")
	}

	// Read exactly `length` bytes
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return frame, err
	}

	// First byte is message type
	frame.Type = payload[0]
	frame.Payload = payload[1:]

	return frame, nil
}
