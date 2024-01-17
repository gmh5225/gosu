package task

import (
	"fmt"
	"time"

	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v3/process"
)

type Report struct {
	Pid        []int32   `json:"pid"`
	Cpu        float64   `json:"cpu"`
	Mem        float64   `json:"mem"`
	Username   string    `json:"usr"`
	CreateTime time.Time `json:"create_time"`
}

func (r Report) String() string {
	if r.IsZero() {
		return "[Not running]"
	} else {
		return fmt.Sprintf(
			"[%s-%d | CPU: %.2f%% MEM: %.2fMB, UP: %s",
			r.Username, r.Pid, r.Cpu, r.Mem/1024/1024, time.Since(r.CreateTime).Round(time.Second),
		)
	}
}

func (r Report) IsZero() bool {
	return len(r.Pid) == 0
}

func fillReportRecursively(p *process.Process, into *Report) {
	if p == nil || p.Pid == 0 || lo.Contains(into.Pid, p.Pid) {
		return
	}
	into.Pid = append(into.Pid, p.Pid)

	if into.Username == "" {
		into.Username, _ = p.Username()
	}
	if into.CreateTime.IsZero() {
		epo, err := p.CreateTime()
		if err == nil {
			into.CreateTime = time.UnixMilli(epo)
		}
	}
	if c, err := p.CPUPercent(); err == nil {
		into.Cpu += c
	}
	if mi, err := p.MemoryInfo(); err == nil && mi != nil {
		into.Mem += float64(mi.RSS)
	}
	if children, err := p.Children(); err == nil {
		for _, child := range children {
			fillReportRecursively(child, into)
		}
	}
}

func InspectProcess(proc *process.Process) (r Report) {
	if proc == nil || proc.Pid == 0 {
		return
	}
	fillReportRecursively(proc, &r)
	return
}
