package session

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/can1357/gosu/pkg/job"
	"github.com/can1357/gosu/pkg/settings"
	"github.com/can1357/gosu/pkg/surpc"
	"github.com/can1357/gosu/pkg/util"
	"github.com/dgraph-io/badger/v4"
	"github.com/shirou/gopsutil/v3/process"
)

type Session struct {
	Database  *badger.DB
	RpcServer *surpc.Server

	Jobs          sync.Map
	JobCollection Collection[string, job.Manifest]

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelCauseFunc
}

var ErrAlreadyExists = errors.New("job already exists")
var ErrNotFound = errors.New("job not found")

func (s *Session) InsertJob(j *job.Job) error {
	if _, loaded := s.Jobs.LoadOrStore(j.ID, j); loaded {
		return ErrAlreadyExists
	}
	s.JobCollection.Replace(j.ID, *j.Manifest)
	j.Ready(s.ctx)
	return nil
}
func (s *Session) UpdateJob(j *job.Job) {
	if prev, loaded := s.Jobs.Swap(j.ID, j); loaded {
		prev.(*job.Job).Stop()
	}
	s.JobCollection.Replace(j.ID, *j.Manifest)
	j.Ready(s.ctx)
}
func (s *Session) DeleteJob(id string) error {
	s.JobCollection.Delete(id)
	val, deleted := s.Jobs.LoadAndDelete(id)
	if deleted {
		val.(*job.Job).Stop()
		return nil
	} else {
		return ErrNotFound
	}
}
func (s *Session) openDatabase() (err error) {

	var opt badger.Options
	if settings.Service.Get().Ephemeral {
		opt = badger.DefaultOptions("").WithInMemory(true)
	} else {
		opt = badger.DefaultOptions(settings.DataDir.Path())
	}

	s.Database, err = badger.Open(
		opt.
			WithLoggingLevel(badger.WARNING).
			WithValueLogFileSize(16 * 1024 * 1024).
			WithCompactL0OnClose(true).
			WithNumVersionsToKeep(0).
			WithNumLevelZeroTables(2).
			WithNumLevelZeroTablesStall(3),
	)
	if err != nil {
		return
	}
	s.JobCollection.Open(s.Database, "jobs")
	return nil
}
func (s *Session) reviveJobs() {
	s.JobCollection.Range(func(id string, recipe job.Manifest) bool {
		{
			data, _ := json.MarshalIndent(recipe, "", "  ")
			log.Printf("Reviving job %s: %s", id, string(data))
		}
		j, err := recipe.Spawn()
		if err != nil {
			log.Printf("Error spawning job %s: %v", id, err)
		} else {
			s.Jobs.Store(id, j)
			go j.Ready(s.ctx)
		}
		return true
	})
}

func (s *Session) ForEachJobRgx(pattern *regexp.Regexp, cb func(j *job.Job) error) (e error) {
	s.Jobs.Range(func(key, value any) bool {
		if pattern != nil && !pattern.MatchString(key.(string)) {
			return true
		}
		e = cb(value.(*job.Job))
		return e == nil
	})
	return
}
func (s *Session) ForEachJob(match string, cb func(j *job.Job) error) (e error) {
	var pattern *regexp.Regexp
	if match != "" && match != "*" && match != "all" && match != ".*" {
		var err error
		pattern, err = regexp.Compile("(?i)" + match)
		if err != nil {
			return err
		}
	}
	return s.ForEachJobRgx(pattern, cb)
}

func Open() (s *Session) {
	s = &Session{}
	s.ctx, s.cancel = util.WithCancelOrOk(context.Background())

	// Open the database.
	if e := s.openDatabase(); e != nil {
		s.Close(e)
		return
	}

	// Kill processes that belong to previous session.
	if processlist, err := process.Processes(); err == nil {
		for _, proc := range processlist {
			if env, err := proc.Environ(); err == nil {
				for _, e := range env {
					if strings.HasPrefix(e, "GOSU_NS") {
						if parent, err := proc.Parent(); err == nil {
							if running, _ := parent.IsRunning(); running {
								continue
							}
						}
						log.Printf("Killing process %d", proc.Pid)
						proc.Kill()
					}
				}
			}
		}
	}

	// Revive jobs.
	s.reviveJobs()

	// Start the RPC server.
	s.RpcServer = surpc.NewServer()
	s.RpcServer.Register("job", &JobService{s})
	s.RpcServer.Register("daemon", &DaemonService{s})
	s.RpcServer.Register("event", &EventService{s})
	s.RpcServer.Register("whiteboard", &WhiteboardService{s})
	s.RpcServer.Router.HandleFunc("/logs", s.LogsHandler)
	if e := s.RpcServer.ListenAll(); e != nil {
		s.Close(e)
		return
	}
	return
}

func (s *Session) StopAll(timeout time.Duration) {
	waitCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	wg := sync.WaitGroup{}
	s.Jobs.Range(func(key, value any) bool {
		j := value.(*job.Job)
		j.Stop()
		wg.Add(1)
		go func() {
			j.Join(waitCtx)
			wg.Done()
		}()
		return true
	})
	wg.Wait()
}

func (s *Session) Wait() error {
	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, syscall.SIGTERM, os.Interrupt)
	select {
	case <-killSignal:
	case <-s.ctx.Done():
	}
	s.StopAll(2 * time.Second)
	s.Close(nil)
	return util.Cause(s.ctx)
}

func (s *Session) Close(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() == nil {
		if s.RpcServer != nil {
			s.RpcServer.Close()
		}
		if s.Database != nil {
			s.Database.Close()
		}
		s.cancel(err)
	}
}
