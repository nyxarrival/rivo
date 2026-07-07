//go:build !windows

package collector

import (
	"fmt"
	"syscall"
)

func readDisk(path string) (uint64, uint64, float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, fmt.Errorf("read disk stat: %w", err)
	}

	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	free := uint64(stat.Bavail) * uint64(stat.Bsize)
	if total == 0 || free > total {
		return 0, 0, 0, fmt.Errorf("read disk stat: invalid total or free disk")
	}

	used := total - free
	return total, used, float64(used) * 100 / float64(total), nil
}
