package protocol

type AppendRequest struct {
	LeaderID          uint32
	PrevIndex         uint64
	PrevChecksum      uint32
	Index             uint64
	Payload           []byte
	LeaderCommitIndex uint64
}

type AppendResponse struct {
	FollowerID uint32
	Success    bool
	LastIndex  uint64
}
