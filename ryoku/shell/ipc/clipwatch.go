package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	wlDisplayID  = 1
	wlRegistryID = 2
	wlCallbackID = 3
	wlSeatID     = 4
	wlManagerID  = 5
	wlDeviceID   = 6

	wlDisplaySyncRequest        = 0
	wlDisplayGetRegistryRequest = 1
	wlDisplayErrorEvent         = 0
	wlDisplayDeleteIDEvent      = 1
	wlRegistryBindRequest       = 0
	wlRegistryGlobalEvent       = 0
	wlCallbackDoneEvent         = 0
	dataManagerGetDeviceRequest = 1
	dataDeviceDataOfferEvent    = 0
	dataDeviceSelectionEvent    = 1
	dataDeviceFinishedEvent     = 2
	dataDevicePrimaryEvent      = 3
	dataOfferReceiveRequest     = 0
	dataOfferDestroyRequest     = 1
	dataOfferMimeEvent          = 0

	extDataControlManager = "ext_data_control_manager_v1"
	wlrDataControlManager = "zwlr_data_control_manager_v1"
	passwordManagerHint   = "x-kde-passwordManagerHint"
	clipTransferTimeout   = 5 * time.Second
)

var (
	errClipWatchNoControl = errors.New("no data-control global")
	errClipWatchFinished  = errors.New("data-control device finished")
)

type wlMessage struct {
	object  uint32
	opcode  uint16
	payload []byte
}

type wlGlobal struct {
	name    uint32
	version uint32
}

type clipOffer struct {
	types []string
}

type clipWatcher struct {
	mu        sync.Mutex
	primaryID uint32
	conn      *net.UnixConn
	ingest    func(string, []byte)
	offers    map[uint32]*clipOffer
}

func (s *clipState) watch() {
	if self, err := os.Executable(); err == nil {
		reapStrayClipWatchers(self)
	}
	for {
		conn, err := dialWayland()
		if err == nil {
			err = runClipWatchSession(conn, s.ingest)
			_ = conn.Close()
		}
		if errors.Is(err, errClipWatchNoControl) {
			log.Printf("ryoku-shell: clipboard history disabled: %v", err)
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func dialWayland() (*net.UnixConn, error) {
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		return nil, errors.New("WAYLAND_DISPLAY is not set")
	}
	path := display
	if !filepath.IsAbs(path) {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			return nil, errors.New("XDG_RUNTIME_DIR is not set")
		}
		path = filepath.Join(runtimeDir, display)
	}
	return net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
}

func runClipWatchSession(conn *net.UnixConn, ingest func(string, []byte)) error {
	watcher := &clipWatcher{conn: conn, ingest: ingest, offers: make(map[uint32]*clipOffer)}
	if err := watcher.send(wlDisplayID, wlDisplayGetRegistryRequest, wlUint(wlRegistryID), -1); err != nil {
		return err
	}
	if err := watcher.send(wlDisplayID, wlDisplaySyncRequest, wlUint(wlCallbackID), -1); err != nil {
		return err
	}

	var seat wlGlobal
	var extManager wlGlobal
	var wlrManager wlGlobal
	for {
		msg, err := readWLMessage(conn)
		if err != nil {
			return err
		}
		if msg.object == wlDisplayID {
			if err := handleDisplayEvent(msg); err != nil {
				return err
			}
			continue
		}
		if msg.object == wlRegistryID && msg.opcode == wlRegistryGlobalEvent {
			name, iface, version, err := decodeGlobal(msg.payload)
			if err != nil {
				return err
			}
			switch iface {
			case "wl_seat":
				if seat.name == 0 {
					seat = wlGlobal{name: name, version: version}
				}
			case extDataControlManager:
				extManager = wlGlobal{name: name, version: version}
			case wlrDataControlManager:
				wlrManager = wlGlobal{name: name, version: version}
			}
			continue
		}
		if msg.object == wlCallbackID && msg.opcode == wlCallbackDoneEvent {
			break
		}
	}
	if seat.name == 0 {
		return errors.New("no wl_seat global")
	}

	manager := extManager
	managerInterface := extDataControlManager
	managerVersion := uint32(1)
	if manager.name == 0 {
		manager = wlrManager
		managerInterface = wlrDataControlManager
		managerVersion = min(manager.version, uint32(2))
	}
	if manager.name == 0 {
		return errClipWatchNoControl
	}
	if err := watcher.bind(seat.name, "wl_seat", min(seat.version, uint32(1)), wlSeatID); err != nil {
		return err
	}
	if managerInterface == extDataControlManager {
		managerVersion = 1
	}
	if err := watcher.bind(manager.name, managerInterface, managerVersion, wlManagerID); err != nil {
		return err
	}
	if err := watcher.send(wlManagerID, dataManagerGetDeviceRequest, append(wlUint(wlDeviceID), wlUint(wlSeatID)...), -1); err != nil {
		return err
	}

	for {
		msg, err := readWLMessage(conn)
		if err != nil {
			return err
		}
		if msg.object == wlDisplayID {
			if err := handleDisplayEvent(msg); err != nil {
				return err
			}
			continue
		}
		if msg.object == wlDeviceID {
			if err := watcher.handleDeviceEvent(msg); err != nil {
				return err
			}
			continue
		}
		watcher.mu.Lock()
		if offer := watcher.offers[msg.object]; offer != nil && msg.opcode == dataOfferMimeEvent {
			mime, _, err := wlString(msg.payload)
			if err != nil {
				watcher.mu.Unlock()
				return err
			}
			offer.types = append(offer.types, mime)
		}
		watcher.mu.Unlock()
	}
}

func (w *clipWatcher) bind(name uint32, iface string, version, id uint32) error {
	payload := wlUint(name)
	payload = append(payload, wlEncodeString(iface)...)
	payload = append(payload, wlUint(version)...)
	payload = append(payload, wlUint(id)...)
	return w.send(wlRegistryID, wlRegistryBindRequest, payload, -1)
}

func (w *clipWatcher) handleDeviceEvent(msg wlMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch msg.opcode {
	case dataDeviceDataOfferEvent:
		id, err := wlFirstUint(msg.payload)
		if err != nil {
			return err
		}
		w.offers[id] = &clipOffer{}
	case dataDeviceSelectionEvent:
		id, err := wlFirstUint(msg.payload)
		if err != nil {
			return err
		}
		if id != 0 {
			return w.receiveSelection(id)
		}
	case dataDeviceFinishedEvent:
		return errClipWatchFinished
	case dataDevicePrimaryEvent:
		id, err := wlFirstUint(msg.payload)
		if err != nil {
			return err
		}
		if previous := w.primaryID; previous != 0 && previous != id {
			if err := w.destroyOffer(previous); err != nil {
				return err
			}
		}
		w.primaryID = id
	}
	return nil
}

func (w *clipWatcher) receiveSelection(id uint32) error {
	offer := w.offers[id]
	if offer == nil {
		return fmt.Errorf("selection refers to unknown offer %d", id)
	}
	for _, mime := range offer.types {
		if mime == passwordManagerHint {
			return w.destroyOffer(id)
		}
	}
	mime := pickBestMime(offer.types)
	if mime == "" {
		return w.destroyOffer(id)
	}

	fds := []int{-1, -1}
	if err := unix.Pipe2(fds, unix.O_CLOEXEC); err != nil {
		_ = w.destroyOffer(id)
		return nil
	}
	readFD, writeFD := fds[0], fds[1]
	if err := unix.SetNonblock(readFD, true); err != nil {
		_ = unix.Close(readFD)
		_ = unix.Close(writeFD)
		_ = w.destroyOffer(id)
		return nil
	}
	payload := wlEncodeString(mime)
	if err := w.send(id, dataOfferReceiveRequest, payload, writeFD); err != nil {
		_ = unix.Close(readFD)
		_ = unix.Close(writeFD)
		return err
	}
	_ = unix.Close(writeFD)
	data, readErr := readClipPipe(readFD, clipMaxEntryBytes+1, clipTransferTimeout)
	_ = unix.Close(readFD)
	if err := w.destroyOffer(id); err != nil {
		return err
	}
	if readErr == nil && len(data) > 0 && len(data) <= clipMaxEntryBytes {
		w.ingest(mime, data)
	}
	return nil
}

func (w *clipWatcher) destroyOffer(id uint32) error {
	delete(w.offers, id)
	return w.send(id, dataOfferDestroyRequest, nil, -1)
}

func readClipPipe(fd, limit int, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	data := make([]byte, 0, min(limit, 64<<10))
	buf := make([]byte, 32<<10)
	for len(data) < limit {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, os.ErrDeadlineExceeded
		}
		wait := int((remaining + time.Millisecond - 1) / time.Millisecond)
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN | unix.POLLHUP | unix.POLLERR}}
		if _, err := unix.Poll(poll, wait); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return nil, err
		}
		if poll[0].Revents == 0 {
			return nil, os.ErrDeadlineExceeded
		}
		n, err := unix.Read(fd, buf[:min(len(buf), limit-len(data))])
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err == nil && n == 0 {
			return data, nil
		}
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func handleDisplayEvent(msg wlMessage) error {
	switch msg.opcode {
	case wlDisplayErrorEvent:
		object, rest, err := wlPopUint(msg.payload)
		if err != nil {
			return err
		}
		code, rest, err := wlPopUint(rest)
		if err != nil {
			return err
		}
		message, _, err := wlString(rest)
		if err != nil {
			return err
		}
		return fmt.Errorf("Wayland protocol error on object %d code %d: %s", object, code, message)
	case wlDisplayDeleteIDEvent:
		_, err := wlFirstUint(msg.payload)
		return err
	default:
		return fmt.Errorf("unknown wl_display event opcode %d", msg.opcode)
	}
}

func decodeGlobal(payload []byte) (uint32, string, uint32, error) {
	name, rest, err := wlPopUint(payload)
	if err != nil {
		return 0, "", 0, err
	}
	iface, rest, err := wlString(rest)
	if err != nil {
		return 0, "", 0, err
	}
	version, _, err := wlPopUint(rest)
	return name, iface, version, err
}

func readWLMessage(r io.Reader) (wlMessage, error) {
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return wlMessage{}, err
	}
	object := binary.NativeEndian.Uint32(header[:4])
	word := binary.NativeEndian.Uint32(header[4:])
	size := int(word >> 16)
	if size < len(header) || size%4 != 0 {
		return wlMessage{}, fmt.Errorf("invalid Wayland message size %d", size)
	}
	payload := make([]byte, size-len(header))
	if _, err := io.ReadFull(r, payload); err != nil {
		return wlMessage{}, err
	}
	return wlMessage{object: object, opcode: uint16(word), payload: payload}, nil
}

func (w *clipWatcher) send(object uint32, opcode uint16, payload []byte, fd int) error {
	msg := wlEncodeMessage(object, opcode, payload)
	if fd < 0 {
		return writeFull(w.conn, msg)
	}
	n, _, err := w.conn.WriteMsgUnix(msg, unix.UnixRights(fd), nil)
	if err != nil {
		return err
	}
	return writeFull(w.conn, msg[n:])
}

func wlEncodeMessage(object uint32, opcode uint16, payload []byte) []byte {
	msg := make([]byte, 8+len(payload))
	binary.NativeEndian.PutUint32(msg[:4], object)
	binary.NativeEndian.PutUint32(msg[4:8], uint32(len(msg))<<16|uint32(opcode))
	copy(msg[8:], payload)
	return msg
}

func wlUint(value uint32) []byte {
	out := make([]byte, 4)
	binary.NativeEndian.PutUint32(out, value)
	return out
}

func wlEncodeString(value string) []byte {
	length := len(value) + 1
	out := make([]byte, 4+((length+3)&^3))
	binary.NativeEndian.PutUint32(out[:4], uint32(length))
	copy(out[4:], value)
	return out
}

func wlFirstUint(payload []byte) (uint32, error) {
	value, _, err := wlPopUint(payload)
	return value, err
}

func wlPopUint(payload []byte) (uint32, []byte, error) {
	if len(payload) < 4 {
		return 0, nil, io.ErrUnexpectedEOF
	}
	return binary.NativeEndian.Uint32(payload[:4]), payload[4:], nil
}

func wlString(payload []byte) (string, []byte, error) {
	length, rest, err := wlPopUint(payload)
	if err != nil {
		return "", nil, err
	}
	padded := (int(length) + 3) &^ 3
	if length == 0 || padded > len(rest) || rest[length-1] != 0 {
		return "", nil, errors.New("invalid Wayland string")
	}
	return string(rest[:length-1]), rest[padded:], nil
}

func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func reapStrayClipWatchers(self string) {
	me := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == me {
			continue
		}
		raw, err := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if err != nil || !isClipWatcherCmdline(strings.Split(string(raw), "\x00"), self) {
			continue
		}
		if process, err := os.FindProcess(pid); err == nil {
			_ = process.Signal(syscall.SIGKILL)
		}
	}
}

func isClipWatcherCmdline(argv []string, self string) bool {
	if len(argv) < 4 || filepath.Base(argv[0]) != "wl-paste" {
		return false
	}
	for i := 1; i+2 < len(argv); i++ {
		if argv[i] == "--watch" && argv[i+1] == self && argv[i+2] == "__clip-ingest" {
			return true
		}
	}
	return false
}
