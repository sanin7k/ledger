package follower

import "github.com/sanin7k/ledger/internal/protocol"

func (f *Follower) HandleAppend(req protocol.AppendRequest) protocol.AppendResponse {
	resp := protocol.AppendResponse{
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
			resp.LastIndex = f.log.LastIndex()
			return resp
		}
	}

	// Probe-only response
	if req.Payload == nil {
		resp.Success = true
		resp.LastIndex = f.log.LastIndex()
		return resp
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
