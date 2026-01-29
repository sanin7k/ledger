package follower

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

func (f *Follower) HandleAppend(req AppendRequest) AppendResponse {
	resp := AppendResponse{
		FollowerID: f.id,
		Success:    false,
		LastIndex:  f.log.LastIndex(),
	}

	if req.PrevIndex > f.log.LastIndex() {
		return resp
	}

	if req.PrevIndex > 0 {
		entry, err := f.log.Read(req.PrevIndex)
		if err != nil {
			print(err.Error())
			return resp
		}

		if entry.Checksum != req.PrevChecksum {
			if err := f.log.TruncateFrom(req.PrevIndex); err != nil {
				print(err.Error())
				return resp
			}
		}
	}

	if req.Index <= f.log.CommitIndex() {
		return resp
	}

	if req.Index != f.log.LastIndex()+1 {
		return resp
	}

	if err := f.log.Append(req.Index, req.Payload); err != nil {
		return resp
	}

	if req.LeaderCommitIndex > f.log.CommitIndex() {
		newCommit := min(req.LeaderCommitIndex, f.log.LastIndex())
		if err := f.log.Commit(newCommit); err != nil {
			return resp
		}
	}

	resp.Success = true
	resp.LastIndex = f.log.LastIndex()
	return resp
}
