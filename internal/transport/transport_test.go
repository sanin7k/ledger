package transport

import (
	"bytes"
	"net"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		err := WriteFrame(c1, Frame{
			Type:    0x01,
			Payload: []byte("hello"),
		})
		if err != nil {
			t.Errorf("write failed: %v", err)
		}
	}()

	frame, err := ReadFrame(c2)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if frame.Type != 0x01 {
		t.Fatalf("unexpected type: %d", frame.Type)
	}
	if !bytes.Equal(frame.Payload, []byte("hello")) {
		t.Fatalf("unexpected payload: %q", frame.Payload)
	}
}

func TestMultipleFrames(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		WriteFrame(c1, Frame{Type: 1, Payload: []byte("A")})
		WriteFrame(c1, Frame{Type: 2, Payload: []byte("B")})
	}()

	f1, err := ReadFrame(c2)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := ReadFrame(c2)
	if err != nil {
		t.Fatal(err)
	}

	if string(f1.Payload) != "A" || string(f2.Payload) != "B" {
		t.Fatalf("frames corrupted")
	}
}

func TestInvalidFrameLength(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		// write invalid length (0)
		c1.Write([]byte{0, 0, 0, 0})
	}()

	_, err := ReadFrame(c2)
	if err == nil {
		t.Fatalf("expected error on invalid frame length")
	}
}

func TestFrameTooLarge(t *testing.T) {
	c1, _ := net.Pipe()
	defer c1.Close()

	err := WriteFrame(c1, Frame{
		Type:    1,
		Payload: make([]byte, MaxFrameSize),
	})

	if err == nil {
		t.Fatalf("expected error for oversized frame")
	}
}
