//go:build windows

package selftest

type syscallStat struct {
	Total uint64
	Free  uint64
}

func getDiskStats(path string, stat *syscallStat) error {
	// Fallback for Windows
	stat.Total = 0
	stat.Free = 0
	return nil
}
