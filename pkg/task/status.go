package task

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Status code for session.
type Status uint8

const (
	FlagTransition Status = 0x20
	FlagAlive      Status = 0x40
	FlagError      Status = 0x80
)

const (
	Idle         Status = iota
	Complete     Status = iota
	Cancelled    Status = iota
	Starting     Status = iota | FlagAlive | FlagTransition
	Stopping     Status = iota | FlagAlive | FlagTransition
	Running      Status = iota | FlagAlive
	Retrying     Status = iota | FlagAlive
	Errored      Status = iota | FlagError
	TimeoutStop  Status = iota | FlagError
	TimeoutStart Status = iota | FlagError
	TimeoutExec  Status = iota | FlagError
)

type statusDetail struct {
	Name  string
	Icon  string
	Error string
}

var statusDetails = map[Status]*statusDetail{
	Idle:         {"idle", "➖", ""},
	Complete:     {"complete", "✔️", ""},
	Cancelled:    {"cancelled", "🚫", "task cancelled"},
	Starting:     {"starting", "🚀", ""},
	Stopping:     {"stopping", "👋", ""},
	Running:      {"running", "🟢", ""},
	Retrying:     {"retrying", "💤", "task is retrying"},
	Errored:      {"errored", "🔴", "task errored"},
	TimeoutStop:  {"timeout-stop", "🕛", "task timed out during exit"},
	TimeoutStart: {"timeout-start", "🕛", "task timed out during launch"},
	TimeoutExec:  {"timeout-exec", "🕛", "task execution timed out"},
}
var statusByName = (func() (res map[string]Status) {
	res = make(map[string]Status)
	for s, d := range statusDetails {
		res[d.Name] = s
	}
	return
})()

func (s Status) detail() *statusDetail {
	return statusDetails[s]
}
func (s Status) IsTransition() bool {
	return (s & FlagTransition) == FlagTransition
}
func (s Status) IsAlive() bool {
	return (s & FlagAlive) == FlagAlive
}
func (s Status) IsDead() bool {
	return !s.IsAlive()
}
func (s Status) IsError() bool {
	return (s & FlagError) == FlagError
}
func (s Status) Icon() string {
	return s.detail().Icon
}
func (s Status) String() string {
	return s.detail().Name
}
func (s Status) Join(err error) error {
	return errors.Join(s, err)
}
func (s Status) Error() string {
	d := s.detail()
	if s.IsError() && d.Error != "" {
		return d.Error
	} else {
		return d.Name
	}
}

func StatusFromErr(err error) Status {
	if err == nil {
		return Complete
	}
	for s := range statusDetails {
		if errors.Is(err, s) {
			return s
		}
	}
	return Errored
}
func StatusFromName(name string) Status {
	return statusByName[name]
}
func (s Status) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, s.String())), nil
}
func (s *Status) UnmarshalJSON(data []byte) (err error) {
	var tmp string
	err = json.Unmarshal(data, &tmp)
	if err != nil {
		return
	}
	*s = StatusFromName(tmp)
	return nil
}
