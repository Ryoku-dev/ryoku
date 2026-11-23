package httpdl

import (
	"errors"
	"fmt"
	"os"

	i18n "ryoku-i18n"

	"golang.org/x/sys/unix"

	"ryoget/internal/proto"
)

func statfsFree(dir string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * st.Bsize, nil
}

// a volume that does not report free space passes
// ported from prism-download-manager crates/prism-engine/src/storage.rs (MIT)
func checkSpace(free func(string) (int64, error), dir string, need int64) error {
	if need <= 0 {
		return nil
	}
	have, err := free(dir)
	if err != nil || have >= need {
		return nil
	}
	return noSpace(need, have)
}

func noSpace(need, have int64) error {
	return &proto.Error{Code: proto.CodeNoSpace, Message: i18n.Tf("Not enough disk space: this download needs %s and %s is free.", sizeText(need), sizeText(have))}
}

func sizeText(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	v, suffix := float64(n)/unit, "KiB"
	for _, s := range []string{"MiB", "GiB", "TiB"} {
		if v < unit {
			break
		}
		v, suffix = v/unit, s
	}
	return fmt.Sprintf("%.1f %s", v, suffix)
}

// filesystems without fallocate get a sparse file
// ported from prism-download-manager crates/prism-engine/src/storage.rs (MIT)
func allocate(f *os.File, size int64) error {
	if size <= 0 {
		return nil
	}
	err := unix.Fallocate(int(f.Fd()), 0, 0, size)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.ENOSPC) {
		have, _ := statfsFree(f.Name())
		return noSpace(size, have)
	}
	return f.Truncate(size)
}

func allocated(path string) int64 {
	var st unix.Stat_t
	if unix.Stat(path, &st) != nil {
		return 0
	}
	return st.Blocks * 512
}

func datasync(f *os.File) error { return unix.Fdatasync(int(f.Fd())) }
