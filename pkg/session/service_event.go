package session

import (
	"github.com/can1357/gosu/pkg/job"
)

type EventService struct {
	session *Session
}

func (s *EventService) Signal(name *string, _ *any) error {
	job.Signal(*name)
	return nil
}
