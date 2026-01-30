package protocol

import (
	"bytes"
	"encoding/binary"
)

func EncodeAppendRequest(req AppendRequest) ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, req.LeaderID); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, req.PrevIndex); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, req.PrevChecksum); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, req.Index); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, req.LeaderCommitIndex); err != nil {
		return nil, err
	}

	payloadLen := uint32(len(req.Payload))
	if err := binary.Write(buf, binary.BigEndian, payloadLen); err != nil {
		return nil, err
	}

	if _, err := buf.Write(req.Payload); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func EncodeAppendResponse(resp AppendResponse) ([]byte, error) {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, resp.FollowerID)
	if resp.Success {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	binary.Write(buf, binary.BigEndian, resp.LastIndex)

	return buf.Bytes(), nil
}
