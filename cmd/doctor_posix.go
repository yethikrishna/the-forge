//go:build !windows

package cmd

import "syscall"

type syscallStat struct {
	stat syscall.Statfs_t
}

func (s *syscallStat) get(path string) error {
	return syscall.Statfs(path, &s.stat)
}

func (s *syscallStat) available() uint64 {
	return s.stat.Bavail * uint64(s.stat.Bsize)
}
