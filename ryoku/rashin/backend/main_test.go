package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

type testGatewaySpawnCall struct {
	Name string
	Args []string
}

var testGatewaySpawns struct {
	sync.Mutex
	Calls []testGatewaySpawnCall
}

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoku-rashin-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create test root:", err)
		os.Exit(2)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "reserve test gateway port:", err)
		_ = os.RemoveAll(root)
		os.Exit(2)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	if err := os.Setenv("RYOKU_PROWL_PORT", strconv.Itoa(port)); err != nil {
		fmt.Fprintln(os.Stderr, "set test gateway port:", err)
		_ = os.RemoveAll(root)
		os.Exit(2)
	}
	if err := os.Setenv("XDG_DATA_HOME", filepath.Join(root, "data")); err != nil {
		fmt.Fprintln(os.Stderr, "set test data home:", err)
		_ = os.RemoveAll(root)
		os.Exit(2)
	}

	gatewaySystemdPresent = func() bool { return false }
	gatewaySpawn = func(name string, args ...string) error {
		testGatewaySpawns.Lock()
		testGatewaySpawns.Calls = append(testGatewaySpawns.Calls, testGatewaySpawnCall{
			Name: name,
			Args: append([]string(nil), args...),
		})
		testGatewaySpawns.Unlock()
		return nil
	}
	// The default daemon port is the user's live Rashin; no test may post to it.
	notifyDaemonChatAgent = func(int, string) {}

	code := m.Run()
	_ = os.RemoveAll(root)
	os.Exit(code)
}
