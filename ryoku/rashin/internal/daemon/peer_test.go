package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// peerHelperSocket tells this test binary, run again as a helper, which socket
// to connect to.
const peerHelperSocket = "RASHIN_PEER_HELPER_SOCKET"

type gateTeams struct {
	core.Teams
	handled []string
}

func (t *gateTeams) Handle(_ context.Context, method string, _ json.RawMessage) (any, error) {
	t.handled = append(t.handled, method)
	return protocol.Empty{}, nil
}

func TestUserOnlyMethodsNeedTheApp(t *testing.T) {
	for _, test := range []struct {
		method string
		gated  bool
	}{
		{protocol.MTeamsApprovalDecide, true},
		{protocol.MTeamsCreate, true},
		{protocol.MTeamsUpdate, true},
		{protocol.MTeamsImport, true},
		{protocol.MTeamsExport, true},
		{protocol.MTeamsTaskCreate, true},
		{protocol.MTeamsTaskRead, true},
		{protocol.MTeamsRoutineCreate, true},
		{protocol.MTeamsRoutineUpdate, true},
		{protocol.MVaultAdd, true},
		{protocol.MVaultUpdate, true},
		{protocol.MVaultRemove, true},
		{protocol.MVaultMail, true},
		{protocol.MVaultMailTest, true},
		{protocol.MTeamsAgentReport, false},
		{protocol.MTeamsAgentHTTP, false},
		{protocol.MTeamsAgentMail, false},
		{protocol.MTeamsAgentRequest, false},
		{protocol.MTeamsList, false},
		{protocol.MTeamsApprovals, false},
		{protocol.MTeamsActivity, false},
		{protocol.MVaultInfo, false},
		{protocol.MVaultEvents, false},
	} {
		t.Run(test.method, func(t *testing.T) {
			crew := &gateTeams{}
			d := &Daemon{deps: Deps{Teams: crew}}
			_, err := d.dispatch(context.Background(), &connection{}, test.method, json.RawMessage(`{}`))
			if !test.gated {
				if err != nil || len(crew.handled) != 1 {
					t.Fatalf("another client calling %s: err %v, handled %v", test.method, err, crew.handled)
				}
				return
			}
			var refused *protocol.Error
			if !errors.As(err, &refused) || refused.Code != protocol.CodeForbidden {
				t.Fatalf("another client calling %s: err %v, want forbidden", test.method, err)
			}
			if len(crew.handled) != 0 {
				t.Fatalf("refused %s still reached Teams", test.method)
			}
			if _, err := d.dispatch(context.Background(), &connection{app: true}, test.method, json.RawMessage(`{}`)); err != nil || len(crew.handled) != 1 {
				t.Fatalf("the app calling %s: err %v, handled %v", test.method, err, crew.handled)
			}
		})
	}
}

func TestProcessExeNamesTheRunningBinary(t *testing.T) {
	want, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	got, err := processExe(int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("processExe = %q, want %q", got, want)
	}
}

func TestAppPathsAreBesideTheDaemonAndPackaged(t *testing.T) {
	allowed := appPaths("/opt/rashin/bin/rashind")
	for _, path := range []string{"/opt/rashin/bin/rashin-app", packagedApp} {
		if !allowed[path] {
			t.Fatalf("%s is not allowed", path)
		}
	}
	for _, path := range []string{"/opt/rashin/bin/rashind", "/opt/rashin/bin/rashin", "/opt/rashin/rashin-app"} {
		if allowed[path] {
			t.Fatalf("%s is allowed", path)
		}
	}
	if only := appPaths(""); !only[packagedApp] || len(only) > 2 {
		t.Fatalf("no daemon path allows %v", only)
	}
}

func TestAppPeerAcceptsTheAppBesideTheDaemon(t *testing.T) {
	dir := t.TempDir()
	app := installHelper(t, dir, appProgram)
	if !peerVerdict(t, app, filepath.Join(dir, "rashind"), false) {
		t.Fatal("rashin-app beside the daemon was refused")
	}
}

func TestAppPeerAcceptsAnUpgradedApp(t *testing.T) {
	dir := t.TempDir()
	app := installHelper(t, dir, appProgram)
	if !peerVerdict(t, app, filepath.Join(dir, "rashind"), true) {
		t.Fatal("rashin-app replaced on disk was refused")
	}
}

func TestAppPeerRefusesAnotherBinary(t *testing.T) {
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if peerVerdict(t, self, filepath.Join(dir, "rashind"), false) {
		t.Fatal("a binary that is not rashin-app was accepted")
	}
	elsewhere := installHelper(t, t.TempDir(), appProgram)
	if peerVerdict(t, elsewhere, filepath.Join(dir, "rashind"), false) {
		t.Fatal("a rashin-app away from the daemon was accepted")
	}
	renamed := installHelper(t, dir, "rashin")
	if peerVerdict(t, renamed, filepath.Join(dir, "rashind"), false) {
		t.Fatal("another binary beside the daemon was accepted")
	}
}

// TestPeerHelperProcess is the peer the appPeer tests start: this test binary
// under another name, holding one connection open until the test closes it.
func TestPeerHelperProcess(t *testing.T) {
	socket := os.Getenv(peerHelperSocket)
	if socket == "" {
		t.Skip("run by the appPeer tests")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		os.Exit(2)
	}
	_, _ = conn.Read(make([]byte, 1))
	os.Exit(0)
}

// installHelper puts this test binary in dir under name, linked when it can
// be so nothing is copied.
func installHelper(t *testing.T, dir, name string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if os.Link(self, path) == nil {
		return path
	}
	source, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// peerVerdict starts binary as a peer that connects to a socket and reports
// whether a daemon started from program takes it for the Rashin app. With
// remove the binary is deleted once the peer runs, as an upgrade replaces it.
func peerVerdict(t *testing.T, binary, program string, remove bool) bool {
	t.Helper()
	isApp := appPeer(program)
	socket := filepath.Join(t.TempDir(), "s")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cmd := exec.Command(binary, "-test.run=^TestPeerHelperProcess$")
	cmd.Env = append(os.Environ(), peerHelperSocket+"="+socket)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if err := listener.(*net.UnixListener).SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	pid, err := verifyPeer(conn)
	if err != nil {
		t.Fatal(err)
	}
	if pid != int32(cmd.Process.Pid) {
		t.Fatalf("peer pid %d, want %d", pid, cmd.Process.Pid)
	}
	if remove {
		if err := os.Remove(binary); err != nil {
			t.Fatal(err)
		}
	}
	return isApp(pid)
}
