package protocol

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestDecodeAppendRequest(t *testing.T) {
	orig := AppendRequest{
		LeaderID:          1,
		PrevIndex:         10,
		PrevChecksum:      12345,
		Index:             11,
		LeaderCommitIndex: 9,
		Payload:           []byte("hello"),
	}

	// Manually encode (mirrors leader-side behavior)
	buf := new(bytes.Buffer)

	write := func(v interface{}) {
		if err := binary.Write(buf, binary.BigEndian, v); err != nil {
			t.Fatal(err)
		}
	}

	write(orig.LeaderID)
	write(orig.PrevIndex)
	write(orig.PrevChecksum)
	write(orig.Index)
	write(orig.LeaderCommitIndex)

	payloadLen := uint32(len(orig.Payload))
	write(payloadLen)
	buf.Write(orig.Payload)

	decoded, err := DecodeAppendRequest(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !reflect.DeepEqual(orig, decoded) {
		t.Fatalf("mismatch:\nexpected %+v\ngot      %+v", orig, decoded)
	}
}

func TestDecodeAppendRequest_TruncatedPayload(t *testing.T) {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, uint32(1))  // LeaderID
	binary.Write(buf, binary.BigEndian, uint64(0))  // PrevIndex
	binary.Write(buf, binary.BigEndian, uint32(0))  // PrevChecksum
	binary.Write(buf, binary.BigEndian, uint64(1))  // Index
	binary.Write(buf, binary.BigEndian, uint64(0))  // CommitIndex
	binary.Write(buf, binary.BigEndian, uint32(10)) // payload len

	// only write 3 bytes instead of 10
	buf.Write([]byte("abc"))

	_, err := DecodeAppendRequest(buf.Bytes())
	if err == nil {
		t.Fatalf("expected error on truncated payload")
	}
}

func TestEncodeAppendResponse(t *testing.T) {
	resp := AppendResponse{
		FollowerID: 7,
		Success:    true,
		LastIndex:  42,
	}

	b, err := EncodeAppendResponse(resp)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	r := bytes.NewReader(b)

	var got AppendResponse
	if err := binary.Read(r, binary.BigEndian, &got.FollowerID); err != nil {
		t.Fatal(err)
	}

	successByte, err := r.ReadByte()
	if err != nil {
		t.Fatal(err)
	}
	got.Success = successByte == 1

	if err := binary.Read(r, binary.BigEndian, &got.LastIndex); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(resp, got) {
		t.Fatalf("mismatch:\nexpected %+v\ngot      %+v", resp, got)
	}
}
