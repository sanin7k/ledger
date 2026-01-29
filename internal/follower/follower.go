package follower

import "github.com/sanin7k/ledger/internal/log"

type Follower struct {
	id  uint32
	log *log.Log
}

func NewFollower(id uint32, log *log.Log) *Follower {
	return &Follower{
		id:  id,
		log: log,
	}
}
