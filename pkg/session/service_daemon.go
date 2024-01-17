package session

import "time"

type DaemonService struct {
	session *Session
}

func (s *DaemonService) Shutdown(_ *any, _ *any) error {
	s.session.StopAll(5 * time.Second)
	s.session.Close(nil)
	return nil
}
func (s *DaemonService) Ping(x, y *int) error {
	*y = *x
	return nil
}
