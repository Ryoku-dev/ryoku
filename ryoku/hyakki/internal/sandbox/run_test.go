package sandbox

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func execOutput(argv []string) (string, error) {
	out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	return string(out), err
}

// TestMain lets the test binary stand in for the rashin command's scope, and
// for a client that dials an abstract socket, inside a sandbox.
func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == "scope" && os.Args[2] == "--" {
		err := Exec(os.Args[3], os.Args[3:], os.Environ(), syscall.Exec)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(126)
	}
	if len(os.Args) == 3 && os.Args[1] == "dial-abstract" {
		conn, err := net.Dial("unix", "@"+os.Args[2])
		switch {
		case err == nil:
			conn.Close()
			fmt.Println("connected")
		case errors.Is(err, syscall.EPERM):
			fmt.Println("refused")
		default:
			fmt.Println("failed:", err)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// The host's /run is empty inside: no system bus, no Docker or libvirt
// socket. What /etc/resolv.conf names still reads, and so does the private
// runtime dir's own socket where it is placed.
func TestRunIsEmptyButResolvAndTheAgentSocketShow(t *testing.T) {
	root, spec := tree(t)
	spec.RuntimeDir = filepath.Join("/run/user", strconv.Itoa(os.Getuid()))
	if _, err := os.Stat(spec.RuntimeDir); err != nil {
		t.Skipf("no runtime dir under /run here: %v", err)
	}
	listener, err := net.Listen("unix", filepath.Join(root, "agent.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	agent := filepath.Join(spec.RuntimeDir, "rashin", "rashin.sock")
	spec.Binds = []Bind{{Source: filepath.Join(root, "agent.sock"), Target: agent}}
	host := []string{"/run/dbus/system_bus_socket", "/run/docker.sock", "/run/libvirt/libvirt-sock", "/run/systemd/private"}
	got := probe(t, spec, `for p in "$@"; do if [ -e "$p" ]; then printf '%s ' "$p"; fi; done; printf '|'; cat /etc/resolv.conf | head -c 64 | wc -c`, append([]string{agent}, host...)...)
	shown, size, _ := strings.Cut(got, "|")
	if strings.TrimSpace(shown) != agent {
		t.Fatalf("inside, of the agent socket and the host's daemons, these exist: %q", shown)
	}
	want, err := os.ReadFile("/etc/resolv.conf")
	if err == nil && len(want) > 0 && strings.TrimSpace(size) == "0" {
		t.Fatal("/etc/resolv.conf reads empty inside")
	}
}

// The file resolv.conf leads to under the run dir shows again, each link of
// the way as a link, and so does each shown path that exists: a link as a
// link, a folder read-only. Nothing outside the run dir is bound again.
func TestRunMountsFollowResolvConfIntoRun(t *testing.T) {
	root := t.TempDir()
	run := mkdir(t, root, "run")
	etc := mkdir(t, root, "etc")
	stub := filepath.Join(mkdir(t, run, "systemd", "resolve"), "stub-resolv.conf")
	writeFile(t, stub, "nameserver 127.0.0.53\n")
	middle := filepath.Join(mkdir(t, run, "resolvconf"), "resolv.conf")
	if err := os.Symlink("../systemd/resolve/stub-resolv.conf", middle); err != nil {
		t.Fatal(err)
	}
	resolv := filepath.Join(etc, "resolv.conf")
	if err := os.Symlink(middle, resolv); err != nil {
		t.Fatal(err)
	}
	system := filepath.Join(run, "current-system")
	if err := os.Symlink("/nix/store/abc-system", system); err != nil {
		t.Fatal(err)
	}
	wrappers := mkdir(t, run, "wrappers")
	mounts := runMounts(resolv, run, []string{system, wrappers, filepath.Join(run, "opengl-driver")})
	var got []string
	for _, m := range mounts {
		got = append(got, strings.Join(m.args, " "))
	}
	want := []string{
		"--symlink ../systemd/resolve/stub-resolv.conf " + middle,
		"--ro-bind " + stub + " " + stub,
		"--symlink /nix/store/abc-system " + system,
		"--ro-bind " + wrappers + " " + wrappers,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("run mounts\n%q\nwant\n%q", got, want)
	}
	if plain := runMounts(stub, filepath.Join(root, "elsewhere"), nil); len(plain) != 0 {
		t.Fatalf("a resolv.conf outside the run dir was bound again: %v", plain)
	}
}

// Through the scope, a program inside cannot reach an abstract socket a
// program outside listens on; without it, it can, which is what the scope is
// for. Kernels without Landlock's scopes skip.
func TestTheScopeKeepsAbstractSocketsOut(t *testing.T) {
	_, spec := tree(t)
	if !Scoped() {
		t.Skip("this kernel's Landlock cannot scope abstract sockets")
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	spec.ReadOnly = []string{filepath.Dir(program)}
	name := fmt.Sprintf("rashin-sandbox-test-%d", os.Getpid())
	listener, err := net.Listen("unix", "@"+name)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	dial := func(scoped bool) string {
		spec := spec
		if scoped {
			spec.Scope = program
		}
		argv, err := Command(spec, []string{program, "dial-abstract", name})
		if err != nil {
			t.Fatal(err)
		}
		out, err := execOutput(argv)
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return strings.TrimSpace(out)
	}
	if got := dial(false); got != "connected" {
		t.Fatalf("unscoped, the dial said %q", got)
	}
	if got := dial(true); got != "refused" {
		t.Fatalf("scoped, the dial said %q", got)
	}
}

// resetScoped forgets what Scoped learned and stands in for the kernel's ABI
// and the thread's restriction until the test ends.
func resetScoped(t *testing.T, abi int, restrict func() error) {
	savedABI, savedRestrict := landlockABI, restrictThread
	landlockABI = func() int { return abi }
	restrictThread = restrict
	scopes.once, scopes.ok = sync.Once{}, false
	t.Cleanup(func() {
		landlockABI, restrictThread = savedABI, savedRestrict
		scopes.once, scopes.ok = sync.Once{}, false
	})
}

// A kernel that can scope but refuses runs nothing; one that cannot scope at
// all runs the program unscoped, and Scoped says so.
func TestTheScopeRefusesToRunWhatItCannotScope(t *testing.T) {
	ran := 0
	run := func(string, []string, []string) error { ran++; return nil }

	resetScoped(t, 6, func() error { return syscall.EPERM })
	if err := Exec("/bin/true", []string{"true"}, nil, run); err == nil || !strings.Contains(err.Error(), "could not be scoped") || ran != 0 {
		t.Fatalf("a refused scope: %v, ran %d", err, ran)
	}

	restricted := 0
	resetScoped(t, 5, func() error { restricted++; return nil })
	if Scoped() {
		t.Fatal("ABI 5 reads as scoped")
	}
	if err := Exec("/bin/true", []string{"true"}, nil, run); err != nil || ran != 1 || restricted != 0 {
		t.Fatalf("an old kernel: %v, ran %d, restricted %d", err, ran, restricted)
	}

	resetScoped(t, 6, func() error { restricted++; return nil })
	if err := Exec("/bin/true", []string{"true"}, nil, run); err != nil || ran != 2 || restricted != 1 {
		t.Fatalf("a scoped run: %v, ran %d, restricted %d", err, ran, restricted)
	}
}
