package git

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"ryoku-rashin/internal/protocol"
)

func ports(worktree string) ([]protocol.Port, error) {
	root, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	listening, err := listeningSockets()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	result := make([]protocol.Port, 0)
	seen := make(map[string]bool)
	uid := uint32(os.Getuid())
	for _, entry := range entries {
		pid, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || !entry.IsDir() {
			continue
		}
		procPath := filepath.Join("/proc", entry.Name())
		info, statErr := os.Stat(procPath)
		if statErr != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uid {
			continue
		}
		cwd, cwdErr := os.Readlink(filepath.Join(procPath, "cwd"))
		if cwdErr != nil || !insidePath(root, cwd) {
			continue
		}
		fds, fdErr := os.ReadDir(filepath.Join(procPath, "fd"))
		if fdErr != nil {
			continue
		}
		commandBytes, _ := os.ReadFile(filepath.Join(procPath, "comm"))
		command := strings.TrimSpace(string(commandBytes))
		for _, fd := range fds {
			target, linkErr := os.Readlink(filepath.Join(procPath, "fd", fd.Name()))
			if linkErr != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			port, found := listening[inode]
			key := fmt.Sprintf("%d:%d", pid, port)
			if !found || seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, protocol.Port{Port: port, PID: pid, Command: command, URL: fmt.Sprintf("http://localhost:%d", port)})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Port != result[j].Port {
			return result[i].Port < result[j].Port
		}
		return result[i].PID < result[j].PID
	})
	return result, nil
}

func insidePath(root, candidate string) bool {
	candidate, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	return candidate == root || strings.HasPrefix(candidate, root+string(filepath.Separator))
}

func listeningSockets() (map[string]int, error) {
	result := make(map[string]int)
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		file, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 10 || fields[3] != "0A" {
				continue
			}
			_, portText, found := strings.Cut(fields[1], ":")
			if !found {
				continue
			}
			port, parseErr := strconv.ParseInt(portText, 16, 32)
			if parseErr == nil {
				result[fields[9]] = int(port)
			}
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return nil, scanErr
		}
	}
	return result, nil
}
