package session

import (
	"encoding/json"

	"github.com/can1357/gosu/pkg/job"
)

type WhiteboardService struct {
	session *Session
}

type RpcWhiteboardKey struct {
	Job string `json:"job,omitempty"`
	Key string `json:"key"`
}
type RpcWhiteboardKv struct {
	RpcWhiteboardKey
	Value json.RawMessage `json:"value,omitempty"`
}

func (s *WhiteboardService) Get(key *RpcWhiteboardKey, out *[]RpcWhiteboardKv) error {
	err := s.session.ForEachJob(key.Job, func(j *job.Job) error {
		wb := j.Whiteboard.Load()
		if wb == nil {
			return nil
		}
		var tmp json.RawMessage
		if err := wb.Get(key.Key, &tmp); err == nil {
			*out = append(*out, RpcWhiteboardKv{
				RpcWhiteboardKey: RpcWhiteboardKey{
					Job: j.ID,
					Key: key.Key,
				},
				Value: tmp,
			})
		}
		return nil
	})
	return err
}
func (s *WhiteboardService) Put(key *RpcWhiteboardKv, count *int) error {
	return s.session.ForEachJob(key.Job, func(j *job.Job) error {
		wb := j.Whiteboard.Load()
		if wb == nil {
			return nil
		}
		wb.Set(key.Key, key.Value)
		*count++
		return nil
	})
}
