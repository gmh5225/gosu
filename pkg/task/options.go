package task

import (
	"time"

	"github.com/can1357/gosu/pkg/util"
)

type Options struct {
	RetryDisabled     bool                  `json:"retry_disabled,omitempty"`      // If set no restarts will be attempted if the process fails to start initially.
	RetryBackoff      util.ParsableDuration `json:"retry_backoff,omitempty"`       // The time to wait between retries.
	RetryBackoffScale float64               `json:"retry_backoff_scale,omitempty"` // The scale factor to apply to the retry backoff.
	RetrySuccess      bool                  `json:"retry_success,omitempty"`       // If set the process will be restarted even if it exits with status 0.
	RetryLimit        util.TimerRate        `json:"retry_limit,omitempty"`         // Maximum number of consequtive restarts within the period before the process is considered "errored".
	MaxMemory         util.ParsableSize     `json:"max_memory,omitempty"`          // Maximum amount of memory the process is allowed to use, <= 0 means unlimited.
	MinUptime         util.ParsableDuration `json:"min_uptime,omitempty"`          // Minimum uptime of the process before it is considered "started", <= 0 means immediate.
	ExecTimeout       util.ParsableDuration `json:"exec_timeout,omitempty"`        // The time to wait for a process to exit before killing it, <= 0 means never.
	StartTimeout      util.ParsableDuration `json:"start_timeout,omitempty"`       // The time to wait for a process to start before killing it, <= 0 means never.
	StopTimeout       util.ParsableDuration `json:"stop_timeout"`                  // The time to wait for a process to stop before killing it, <= 0 means immediate.
}

func (o *Options) WithDefaults() {
	if o.RetryLimit.IsZero() {
		o.RetryLimit = util.Rate(10, 30*time.Second)
	} else {
		o.RetryLimit = o.RetryLimit.
			ClampPeriod(30*time.Second, time.Hour).
			Clamp(util.Rate(0.0), util.Rate(10, 30*time.Second))
	}

	if o.RetryBackoffScale <= 0 {
		o.RetryBackoffScale = 2
	}
	o.RetryBackoff = o.RetryBackoff.Max(500 * time.Millisecond)
	if o.StopTimeout.IsPositive() {
		o.StopTimeout = o.StopTimeout.Clamp(250*time.Millisecond, 30*time.Second)
	} else {
		o.StopTimeout.Duration = 5 * time.Second
	}
	if !o.MinUptime.IsPositive() {
		o.MinUptime = util.Duration(0)
	}
}
