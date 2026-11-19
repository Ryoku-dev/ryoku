package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type capturedClip struct {
	mime string
	data []byte
}

func TestClipWatcherCapturesOffersInOrder(t *testing.T) {
	client, server := clipSocketPair(t)
	defer client.Close()
	defer server.Close()

	captured := make(chan capturedClip, 8)
	result := make(chan error, 1)
	go func() {
		result <- runClipWatchSession(client, func(mime string, data []byte) {
			captured <- capturedClip{mime: mime, data: append([]byte(nil), data...)}
		})
	}()
	if manager := fakeClipHandshake(t, server, true); manager != extDataControlManager {
		t.Fatalf("bound manager = %q, want %q", manager, extDataControlManager)
	}

	fakeSendOffer(t, server, 0xff000001, []string{
		"chromium/x-source-url", "text/plain", "text/plain;charset=utf-8",
	}, dataDeviceSelectionEvent)
	fakeServeReceive(t, server, 0xff000001, "text/plain;charset=utf-8", []byte("alpha"))

	fakeSendOffer(t, server, 0xff000002, []string{"text/html", "image/png"}, dataDeviceSelectionEvent)
	fakeServeReceive(t, server, 0xff000002, "image/png", []byte("png-bytes"))

	fakeSendOffer(t, server, 0xff000003, []string{passwordManagerHint, "text/plain"}, dataDeviceSelectionEvent)
	fakeExpectDestroy(t, server, 0xff000003)

	fakeSendOffer(t, server, 0xff000004, []string{"chromium/x-source-url"}, dataDeviceSelectionEvent)
	fakeExpectDestroy(t, server, 0xff000004)

	fakeSendOffer(t, server, 0xff000005, []string{"text/plain"}, dataDevicePrimaryEvent)
	fakeSendOffer(t, server, 0xff000009, []string{"text/plain"}, dataDevicePrimaryEvent)
	fakeExpectDestroy(t, server, 0xff000005)
	fakeSend(t, server, wlDeviceID, dataDevicePrimaryEvent, wlUint(0))
	fakeExpectDestroy(t, server, 0xff000009)

	packet := fakeOfferPacket(0xff000006, []string{"text/plain"}, dataDeviceSelectionEvent)
	packet = append(packet, fakeOfferPacket(0xff000007, []string{"image/jpeg"}, dataDeviceSelectionEvent)...)
	if err := writeFull(server, packet); err != nil {
		t.Fatal(err)
	}
	fakeServeReceive(t, server, 0xff000006, "text/plain", []byte("first queued"))
	fakeServeReceive(t, server, 0xff000007, "image/jpeg", []byte("second queued"))

	fakeSendOffer(t, server, 0xff000008, []string{"application/octet-stream"}, dataDeviceSelectionEvent)
	fakeServeReceive(t, server, 0xff000008, "application/octet-stream", bytes.Repeat([]byte{'x'}, clipMaxEntryBytes+1))

	want := []capturedClip{
		{mime: "text/plain;charset=utf-8", data: []byte("alpha")},
		{mime: "image/png", data: []byte("png-bytes")},
		{mime: "text/plain", data: []byte("first queued")},
		{mime: "image/jpeg", data: []byte("second queued")},
	}
	got := make([]capturedClip, 0, len(want))
	for range want {
		select {
		case clip := <-captured:
			got = append(got, clip)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out after %d captures", len(got))
		}
	}
	select {
	case extra := <-captured:
		t.Fatalf("unexpected capture: mime=%q size=%d", extra.mime, len(extra.data))
	default:
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("captures = %#v, want %#v", got, want)
	}

	fakeSend(t, server, wlDeviceID, dataDeviceFinishedEvent, nil)
	select {
	case err := <-result:
		if !errors.Is(err, errClipWatchFinished) {
			t.Fatalf("watch result = %v, want device finished", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after device finished")
	}
}

func TestClipWatcherFallsBackToWlrDataControl(t *testing.T) {
	client, server := clipSocketPair(t)
	defer client.Close()
	defer server.Close()

	captured := make(chan capturedClip, 1)
	result := make(chan error, 1)
	go func() {
		result <- runClipWatchSession(client, func(mime string, data []byte) {
			captured <- capturedClip{mime: mime, data: append([]byte(nil), data...)}
		})
	}()
	if manager := fakeClipHandshake(t, server, false); manager != wlrDataControlManager {
		t.Fatalf("bound manager = %q, want %q", manager, wlrDataControlManager)
	}
	fakeSendOffer(t, server, 0xff000011, []string{"text/plain"}, dataDeviceSelectionEvent)
	fakeServeReceive(t, server, 0xff000011, "text/plain", []byte("fallback"))
	select {
	case got := <-captured:
		if got.mime != "text/plain" || !bytes.Equal(got.data, []byte("fallback")) {
			t.Fatalf("capture = %#v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fallback capture")
	}
	fakeSend(t, server, wlDeviceID, dataDeviceFinishedEvent, nil)
	if err := <-result; !errors.Is(err, errClipWatchFinished) {
		t.Fatalf("watch result = %v, want device finished", err)
	}
}

func TestClipWatcherStopsWithoutDataControl(t *testing.T) {
	client, server := clipSocketPair(t)
	defer client.Close()
	defer server.Close()

	result := make(chan error, 1)
	go func() { result <- runClipWatchSession(client, func(string, []byte) {}) }()
	fakeReadRequest(t, server, wlDisplayID, wlDisplayGetRegistryRequest)
	fakeReadRequest(t, server, wlDisplayID, wlDisplaySyncRequest)
	fakeSend(t, server, wlRegistryID, wlRegistryGlobalEvent, fakeGlobal(10, "wl_seat", 9))
	fakeSend(t, server, wlCallbackID, wlCallbackDoneEvent, wlUint(0))
	select {
	case err := <-result:
		if !errors.Is(err, errClipWatchNoControl) {
			t.Fatalf("watch result = %v, want no data-control global", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after registry discovery")
	}
}

func TestIsLegacyClipWatcherCmdline(t *testing.T) {
	const self = "/home/u/.local/bin/ryoku-shell"
	cases := []struct {
		name string
		argv []string
		want bool
	}{
		{"watcher", []string{"wl-paste", "--watch", self, "__clip-ingest"}, true},
		{"watcher by path", []string{"/usr/bin/wl-paste", "--watch", self, "__clip-ingest"}, true},
		{"foreign watcher", []string{"wl-paste", "--watch", "/usr/bin/cliphist", "store"}, false},
		{"read", []string{"wl-paste", "-n", "-t", "image/png"}, false},
		{"daemon", []string{self, "daemon"}, false},
	}
	for _, test := range cases {
		if got := isClipWatcherCmdline(test.argv, self); got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
}

func clipSocketPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	wrap := func(fd int, name string) *net.UnixConn {
		file := os.NewFile(uintptr(fd), name)
		conn, err := net.FileConn(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		unixConn, ok := conn.(*net.UnixConn)
		if !ok {
			t.Fatalf("socketpair connection is %T", conn)
		}
		return unixConn
	}
	return wrap(fds[0], "clipwatch-client"), wrap(fds[1], "clipwatch-server")
}

func fakeClipHandshake(t *testing.T, conn *net.UnixConn, advertiseExt bool) string {
	t.Helper()
	fakeReadRequest(t, conn, wlDisplayID, wlDisplayGetRegistryRequest)
	fakeReadRequest(t, conn, wlDisplayID, wlDisplaySyncRequest)
	fakeSend(t, conn, wlRegistryID, wlRegistryGlobalEvent, fakeGlobal(10, "wl_seat", 9))
	fakeSend(t, conn, wlRegistryID, wlRegistryGlobalEvent, fakeGlobal(11, wlrDataControlManager, 2))
	if advertiseExt {
		fakeSend(t, conn, wlRegistryID, wlRegistryGlobalEvent, fakeGlobal(12, extDataControlManager, 1))
	}
	fakeSend(t, conn, wlCallbackID, wlCallbackDoneEvent, wlUint(0))
	fakeSend(t, conn, wlDisplayID, wlDisplayDeleteIDEvent, wlUint(wlCallbackID))

	seatBind := fakeReadRequest(t, conn, wlRegistryID, wlRegistryBindRequest)
	seatName, seatInterface, _, seatID := fakeDecodeBind(t, seatBind.payload)
	if seatName != 10 || seatInterface != "wl_seat" || seatID != wlSeatID {
		t.Fatalf("seat bind = (%d, %q, %d)", seatName, seatInterface, seatID)
	}
	managerBind := fakeReadRequest(t, conn, wlRegistryID, wlRegistryBindRequest)
	managerName, managerInterface, managerVersion, managerID := fakeDecodeBind(t, managerBind.payload)
	wantManagerName := uint32(11)
	if advertiseExt {
		wantManagerName = 12
	}
	if managerName != wantManagerName || managerID != wlManagerID {
		t.Fatalf("manager bind = (%d, %d), want (%d, %d)", managerName, managerID, wantManagerName, wlManagerID)
	}
	if managerInterface == extDataControlManager && managerVersion != 1 {
		t.Fatalf("ext manager version = %d, want 1", managerVersion)
	}
	if managerInterface == wlrDataControlManager && managerVersion != 2 {
		t.Fatalf("wlr manager version = %d, want 2", managerVersion)
	}
	device := fakeReadRequest(t, conn, wlManagerID, dataManagerGetDeviceRequest)
	deviceID, rest, err := wlPopUint(device.payload)
	if err != nil {
		t.Fatal(err)
	}
	seatID, _, err = wlPopUint(rest)
	if err != nil {
		t.Fatal(err)
	}
	if deviceID != wlDeviceID || seatID != wlSeatID {
		t.Fatalf("get_data_device = (%d, %d), want (%d, %d)", deviceID, seatID, wlDeviceID, wlSeatID)
	}
	return managerInterface
}

func fakeDecodeBind(t *testing.T, payload []byte) (uint32, string, uint32, uint32) {
	t.Helper()
	name, rest, err := wlPopUint(payload)
	if err != nil {
		t.Fatal(err)
	}
	iface, rest, err := wlString(rest)
	if err != nil {
		t.Fatal(err)
	}
	version, rest, err := wlPopUint(rest)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := wlPopUint(rest)
	if err != nil {
		t.Fatal(err)
	}
	return name, iface, version, id
}

func fakeGlobal(name uint32, iface string, version uint32) []byte {
	payload := wlUint(name)
	payload = append(payload, wlEncodeString(iface)...)
	return append(payload, wlUint(version)...)
}

func fakeSendOffer(t *testing.T, conn *net.UnixConn, id uint32, types []string, selectionOpcode uint16) {
	t.Helper()
	if err := writeFull(conn, fakeOfferPacket(id, types, selectionOpcode)); err != nil {
		t.Fatal(err)
	}
}

func fakeOfferPacket(id uint32, types []string, selectionOpcode uint16) []byte {
	packet := wlEncodeMessage(wlDeviceID, dataDeviceDataOfferEvent, wlUint(id))
	for _, mime := range types {
		packet = append(packet, wlEncodeMessage(id, dataOfferMimeEvent, wlEncodeString(mime))...)
	}
	return append(packet, wlEncodeMessage(wlDeviceID, selectionOpcode, wlUint(id))...)
}

func fakeServeReceive(t *testing.T, conn *net.UnixConn, offerID uint32, wantMime string, data []byte) {
	t.Helper()
	msg, fd := fakeReadFDRequest(t, conn)
	if msg.object != offerID || msg.opcode != dataOfferReceiveRequest {
		_ = unix.Close(fd)
		t.Fatalf("receive request = object %d opcode %d, want object %d opcode 0", msg.object, msg.opcode, offerID)
	}
	mime, _, err := wlString(msg.payload)
	if err != nil {
		_ = unix.Close(fd)
		t.Fatal(err)
	}
	if mime != wantMime {
		_ = unix.Close(fd)
		t.Fatalf("receive mime = %q, want %q", mime, wantMime)
	}
	file := os.NewFile(uintptr(fd), fmt.Sprintf("offer-%d", offerID))
	if err := writeFull(file, data); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	fakeExpectDestroy(t, conn, offerID)
}

func fakeExpectDestroy(t *testing.T, conn *net.UnixConn, offerID uint32) {
	t.Helper()
	msg := fakeReadRequest(t, conn, offerID, dataOfferDestroyRequest)
	if len(msg.payload) != 0 {
		t.Fatalf("destroy payload length = %d, want 0", len(msg.payload))
	}
}

func fakeReadRequest(t *testing.T, conn *net.UnixConn, object uint32, opcode uint16) wlMessage {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	msg, err := readWLMessage(conn)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if msg.object != object || msg.opcode != opcode {
		t.Fatalf("request = object %d opcode %d, want object %d opcode %d", msg.object, msg.opcode, object, opcode)
	}
	return msg
}

func fakeReadFDRequest(t *testing.T, conn *net.UnixConn) (wlMessage, int) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 65535)
	oob := make([]byte, unix.CmsgSpace(4))
	n, oobn, _, _, err := conn.ReadMsgUnix(data, oob)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if n < 8 {
		t.Fatalf("fd request length = %d", n)
	}
	size := int(binary.NativeEndian.Uint32(data[4:8]) >> 16)
	for n < size {
		more, err := conn.Read(data[n:size])
		if err != nil {
			t.Fatal(err)
		}
		n += more
	}
	msg, err := readWLMessage(bytes.NewReader(data[:size]))
	if err != nil {
		t.Fatal(err)
	}
	control, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatal(err)
	}
	var rights []int
	for _, message := range control {
		fds, err := unix.ParseUnixRights(&message)
		if err != nil {
			t.Fatal(err)
		}
		rights = append(rights, fds...)
	}
	if len(rights) != 1 {
		for _, fd := range rights {
			_ = unix.Close(fd)
		}
		t.Fatalf("fd request carried %d descriptors, want 1", len(rights))
	}
	return msg, rights[0]
}

func fakeSend(t *testing.T, conn *net.UnixConn, object uint32, opcode uint16, payload []byte) {
	t.Helper()
	if err := writeFull(conn, wlEncodeMessage(object, opcode, payload)); err != nil {
		t.Fatal(err)
	}
}
