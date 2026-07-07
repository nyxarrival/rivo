//go:build windows

package collector

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func readDisk(path string) (uint64, uint64, float64, error) {
	diskPath := normalizeWindowsDiskPath(path)
	diskPathPtr, err := windows.UTF16PtrFromString(diskPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read disk stat: %w", err)
	}

	var freeBytesAvailable uint64
	var total uint64
	var free uint64
	if err := windows.GetDiskFreeSpaceEx(diskPathPtr, &freeBytesAvailable, &total, &free); err != nil {
		return 0, 0, 0, fmt.Errorf("read disk stat: %w", err)
	}
	if total == 0 || free > total {
		return 0, 0, 0, fmt.Errorf("read disk stat: invalid total or free disk")
	}

	used := total - free
	return total, used, float64(used) * 100 / float64(total), nil
}

func normalizeWindowsDiskPath(path string) string {
	if path == "" || path == "/" {
		if systemDrive := os.Getenv("SystemDrive"); systemDrive != "" {
			return ensureTrailingBackslash(systemDrive)
		}
		return `C:\`
	}
	return ensureTrailingBackslash(path)
}

func ensureTrailingBackslash(path string) string {
	if strings.HasSuffix(path, `\`) || strings.HasSuffix(path, `/`) {
		return path
	}
	return path + `\`
}
