package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
)

func DecodeAppendRequest(b []byte) (AppendRequest, error) {
	buf := bytes.NewReader(b)

	var req AppendRequest

	if err := binary.Read(buf, binary.BigEndian, &req.LeaderID); err != nil {
		return req, err
	}
	if err := binary.Read(buf, binary.BigEndian, &req.PrevIndex); err != nil {
		return req, err
	}
	if err := binary.Read(buf, binary.BigEndian, &req.PrevChecksum); err != nil {
		return req, err
	}
	if err := binary.Read(buf, binary.BigEndian, &req.Index); err != nil {
		return req, err
	}
	if err := binary.Read(buf, binary.BigEndian, &req.LeaderCommitIndex); err != nil {
		return req, err
	}

	var payloadLen uint32
	if err := binary.Read(buf, binary.BigEndian, &payloadLen); err != nil {
		return req, err
	}

	req.Payload = make([]byte, payloadLen)
	n, err := buf.Read(req.Payload)
	if err != nil || uint32(n) != payloadLen {
		return req, errors.New("payload truncated")
	}

	return req, nil
}

func DecodeAppendResponse(b []byte) (AppendResponse, error) {
	var resp AppendResponse

	buf := bytes.NewReader(b)

	if err := binary.Read(buf, binary.BigEndian, &resp.FollowerID); err != nil {
		return resp, err
	}

	successByte, err := buf.ReadByte()
	if err != nil {
		return resp, err
	}
	resp.Success = successByte == 1

	if err := binary.Read(buf, binary.BigEndian, &resp.LastIndex); err != nil {
		return resp, err
	}

	// Defensive: ensure no trailing garbage
	if buf.Len() != 0 {
		return resp, errors.New("extra bytes in AppendResponse")
	}

	return resp, nil
}
