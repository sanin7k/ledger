package protocol

import (
	"bytes"
	"encoding/binary"
)

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
