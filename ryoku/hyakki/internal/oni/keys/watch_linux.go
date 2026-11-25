//go:build linux

package keys

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"syscall"
)

// supported: Linux has evdev.
const supported = true

// watchDir calls plugged with the name of every node created in dir or
// given new permissions (udev grants uaccess just after creating it). The
// inotify descriptor sleeps in the poller; closing it ends the watch.
func watchDir(dir string, plugged func(name string)) (io.Closer, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, err
	}
	if _, err := syscall.InotifyAddWatch(fd, dir, syscall.IN_CREATE|syscall.IN_ATTRIB); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "inotify")
	go func() {
		buf := make([]byte, 16*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
		for {
			n, err := f.Read(buf)
			if err != nil {
				return
			}
			for at := 0; at+syscall.SizeofInotifyEvent <= n; {
				length := int(binary.NativeEndian.Uint32(buf[at+12 : at+16]))
				end := at + syscall.SizeofInotifyEvent + length
				if end > n {
					break
				}
				name := buf[at+syscall.SizeofInotifyEvent : end]
				if i := bytes.IndexByte(name, 0); i >= 0 {
					name = name[:i]
				}
				plugged(string(name))
				at = end
			}
		}
	}()
	return f, nil
}
