//go:build linux || darwin

package selftest

import "syscall"

type syscallStat struct {
	Total uint64
	Free  uint64
}

func getDiskStats(path string, stat *syscallStat) error {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return err
	}
	stat.Total = fs.Blocks * uint64(fs.Bsize)
	stat.Free = fs.Bavail * uint64(fs.Bsize)
	return nil
}
