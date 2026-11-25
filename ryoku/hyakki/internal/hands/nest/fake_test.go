package nest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary stands in for weston, niri, grim and a program: a test
// links it under those names into a bin dir on PATH, and run under one of
// them it records its argv, environment and working dir next to itself and
// acts as the program would for the package. A file mode.<name> beside it
// changes how it behaves.
const daemonEnv = "NEST_TEST_DAEMON_DIR"

var fakeNames = []string{"weston", nestedProviderName, "grim", "fakeapp", "leftover", "bash"}

func TestMain(m *testing.M) {
	name := filepath.Base(os.Args[0])
	for _, fake := range fakeNames {
		if name == fake {
			runFake(name)
			os.Exit(0)
		}
	}
	if dir := os.Getenv(daemonEnv); dir != "" {
		runDaemon(dir)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type record struct {
	Argv []string `json:"argv"`
	Env  []string `json:"env"`
	Cwd  string   `json:"cwd"`
	Pid  int      `json:"pid"`
}

func fakeBin() string {
	exe, _ := os.Executable()
	return filepath.Dir(exe)
}

func fakeMode(name string) string {
	data, _ := os.ReadFile(filepath.Join(fakeBin(), "mode."+name))
	return strings.TrimSpace(string(data))
}

func writeRecord(name string) {
	cwd, _ := os.Getwd()
	data, _ := json.Marshal(record{Argv: os.Args, Env: os.Environ(), Cwd: cwd, Pid: os.Getpid()})
	dir := filepath.Join(fakeBin(), "records")
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.%d", name, os.Getpid()))
	_ = os.WriteFile(tmp, data, 0o600)
	_ = os.Rename(tmp, filepath.Join(dir, fmt.Sprintf("%s.%d", name, os.Getpid())))
}

func sleepForever() {
	for {
		time.Sleep(time.Hour)
	}
}

func listen(path string) {
	l, err := net.Listen("unix", path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake: listen:", err)
		os.Exit(2)
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
}

func runFake(name string) {
	writeRecord(name)
	mode := fakeMode(name)
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	switch name {
	case "weston":
		switch mode {
		case "die":
			fmt.Fprintln(os.Stderr, "fake weston: no backend")
			os.Exit(3)
		case "hang":
			sleepForever()
		case "stubborn":
			stubborn()
		}
		socket := ""
		for _, arg := range os.Args[1:] {
			if value, ok := strings.CutPrefix(arg, "--socket="); ok {
				socket = value
			}
		}
		listen(filepath.Join(runtime, socket))
		sleepForever()
	case nestedProviderName:
		if mode == "die" {
			fmt.Fprintln(os.Stderr, "fake niri: no config")
			os.Exit(4)
		}
		if len(os.Args) != 3 || os.Args[1] != "-c" {
			os.Exit(5)
		}
		if _, err := os.Stat(os.Args[2]); err != nil {
			os.Exit(6)
		}
		if !isSocket(filepath.Join(runtime, os.Getenv("WAYLAND_DISPLAY"))) {
			os.Exit(7)
		}
		if mode == "escape" {
			escaped := exec.Command(filepath.Join(fakeBin(), "leftover"))
			escaped.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			_ = escaped.Start()
		}
		listen(filepath.Join(runtime, "wayland-1"))
		listen(filepath.Join(runtime, fmt.Sprintf("niri.wayland-1.%d.sock", os.Getpid())))
		sleepForever()
	case "grim":
		if mode == "garbage" {
			fmt.Print("not a picture")
			return
		}
		_ = png.Encode(os.Stdout, image.NewGray(image.Rect(0, 0, 4, 4)))
	default:
		sleepForever()
	}
}

// stubborn ignores SIGTERM, as a program that will not end when asked.
func stubborn() { signal.Ignore(syscall.SIGTERM) }

// runDaemon is a daemon that starts a desktop, prints its compositors' pids
// and waits to be killed.
func runDaemon(dir string) {
	w, err := Start(context.Background(), Options{Dir: dir, Home: dir + "-home"})
	if err != nil {
		fmt.Println("error", err)
		return
	}
	fmt.Println(w.weston.cmd.Process.Pid, w.niri.cmd.Process.Pid)
	sleepForever()
}

// fakes makes a bin dir holding the fake programs, puts it alone on PATH
// and sets the given modes.
func fakes(t *testing.T, modes map[string]string, names ...string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Mkdir(filepath.Join(bin, "records"), 0o700); err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		names = fakeNames
	}
	for _, name := range names {
		if err := os.Link(exe, filepath.Join(bin, name)); err != nil {
			copyFile(t, exe, filepath.Join(bin, name))
		}
	}
	for name, mode := range modes {
		if err := os.WriteFile(filepath.Join(bin, "mode."+name), []byte(mode), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return bin
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// records reads every record the named fake wrote.
func records(t *testing.T, bin, name string) []record {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(bin, "records", name+".*"))
	var out []record
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var r record
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// awaitRecord waits for the named fake's single record.
func awaitRecord(t *testing.T, bin, name string) record {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rs := records(t, bin, name); len(rs) == 1 {
			return rs[0]
		} else if len(rs) > 1 {
			t.Fatalf("%s ran %d times", name, len(rs))
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never ran", name)
	return record{}
}

// gone waits for pid to leave the process table; a zombie counts as gone.
func gone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil {
			return
		}
		if fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:])); len(fields) > 0 && fields[0] == "Z" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("process %d is still running", pid)
}

// holders lists the processes whose environment names dir as their runtime
// dir.
func holders(t *testing.T, dir string) []int {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatal(err)
	}
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		environ, err := os.ReadFile("/proc/" + entry.Name() + "/environ")
		if err != nil {
			continue
		}
		for _, kv := range strings.Split(string(environ), "\x00") {
			if kv == "XDG_RUNTIME_DIR="+dir {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

// shortDir is a fresh runtime dir path, short enough for niri's sockets.
func shortDir(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("", "nt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	return filepath.Join(base, "r")
}

// startDaemon runs runDaemon in a child process and reads its pids.
func startDaemon(t *testing.T, dir string) (*exec.Cmd, int, int) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), daemonEnv+"="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(out).ReadString('\n')
	var weston, niri int
	if _, err := fmt.Sscan(line, &weston, &niri); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("daemon said %q", line)
	}
	return cmd, weston, niri
}
