//go:build windows

package cmd

import "syscall"

type syscallStat struct {
	stat syscall.Stat_t
}

func (s *syscallStat) get(path string) error {
	return syscall.Stat(path, &s.stat)
}

func (s *syscallStat) available() uint64 {
	return 10 * 1024 * 1024 * 1024 // 10 GB stub
}
