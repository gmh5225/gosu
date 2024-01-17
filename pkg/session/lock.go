package session

import (
	"path/filepath"

	"github.com/can1357/gosu/pkg/settings"
	"github.com/nightlyone/lockfile"
)

func Lockfile() (lockfile.Lockfile, error) {
	file := filepath.Join(settings.Home(), ".lock")
	return lockfile.New(file)
}

func Running() bool {
	lf, err := Lockfile()
	if err == nil {
		if lf.TryLock() == nil {
			lf.Unlock()
			return false
		}
	}
	return true
}

func TryAcquire() bool {
	lf, err := Lockfile()
	if err != nil {
		return false
	}
	return lf.TryLock() == nil
}
