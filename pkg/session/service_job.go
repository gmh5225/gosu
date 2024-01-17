package session

import (
	"github.com/can1357/gosu/pkg/job"
	"github.com/can1357/gosu/pkg/task"
)

type RpcStatus struct {
	Icon  string `json:"icon"`
	Code  string `json:"code"`
	Error string `json:"error,omitempty"`
}
type RpcTaskInfo struct {
	Namespace string        `json:"namespace"`
	Status    RpcStatus     `json:"status"`
	Report    task.Report   `json:"report,omitempty"`
	Children  []RpcTaskInfo `json:"children,omitempty"`
}
type RpcJobInfo struct {
	ID   string      `json:"id"`
	Main RpcTaskInfo `json:"main"`
}
type RpcSessionJobs struct {
	Jobs []RpcJobInfo `json:"jobs"`
}

func makeRpcStatus(e error) RpcStatus {
	s := task.StatusFromErr(e)
	return RpcStatus{
		Icon:  s.Icon(),
		Code:  s.String(),
		Error: e.Error(),
	}
}

type JobService struct {
	session *Session
}

func (s *JobService) taskInfo(w task.Worker) (t RpcTaskInfo) {
	if w == nil {
		return
	}
	t.Namespace = w.Namespace()
	t.Status = makeRpcStatus(w.Status())
	t.Report = w.Inspect()
	w.Traverse(func(child task.Worker) bool {
		t.Children = append(t.Children, s.taskInfo(child))
		return true
	})
	return
}
func (s *JobService) jobInfo(j *job.Job) (o RpcJobInfo) {
	o.ID = j.ID
	o.Main = s.taskInfo(j.Worker())
	return
}
func (s *JobService) List(match *string, result *RpcSessionJobs) error {
	return s.session.ForEachJob(*match, func(j *job.Job) error {
		(*result).Jobs = append((*result).Jobs, s.jobInfo(j))
		return nil
	})
}
func (s *JobService) Launch(recipe job.Manifest, result *RpcJobInfo) error {
	j, err := recipe.Spawn()
	if err != nil {
		return err
	}
	s.session.UpdateJob(j)
	j.Start()
	*result = s.jobInfo(j)
	return nil
}
func (s *JobService) Update(recipe job.Manifest, result *RpcJobInfo) error {
	j, err := recipe.Spawn()
	if err != nil {
		return err
	}
	s.session.UpdateJob(j)
	*result = s.jobInfo(j)
	return nil
}
func (s *JobService) Delete(match *string, list *[]string) error {
	return s.session.ForEachJob(*match, func(j *job.Job) error {
		s.session.DeleteJob(j.ID)
		*list = append(*list, j.ID)
		return nil
	})
}
func (s *JobService) Start(match *string, list *[]string) error {
	return s.session.ForEachJob(*match, func(j *job.Job) error {
		j.Start()
		*list = append(*list, j.ID)
		return nil
	})
}
func (s *JobService) Stop(match *string, list *[]string) error {
	return s.session.ForEachJob(*match, func(j *job.Job) error {
		j.Stop()
		*list = append(*list, j.ID)
		return nil
	})
}
func (s *JobService) Restart(match *string, list *[]string) error {
	return s.session.ForEachJob(*match, func(j *job.Job) error {
		j.Restart()
		*list = append(*list, j.ID)
		return nil
	})
}
func (s *JobService) Kill(match *string, list *[]string) error {
	return s.session.ForEachJob(*match, func(j *job.Job) error {
		j.Kill()
		*list = append(*list, j.ID)
		return nil
	})
}
