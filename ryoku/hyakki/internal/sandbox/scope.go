package sandbox

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// scopeABI is the first Landlock ABI that scopes abstract Unix sockets and
// signals (Linux 6.12).
const scopeABI = 6

// landlockABI is the kernel's Landlock ABI, 0 where Landlock is off or
// missing. A variable so a test can stand in for the kernel.
var landlockABI = func() int {
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return 0
	}
	return int(abi)
}

var scopes struct {
	once sync.Once
	ok   bool
}

// Scoped reports whether this kernel lets a sandbox keep its programs from
// abstract Unix sockets, such as an X server's, and from signalling
// processes outside it.
func Scoped() bool {
	scopes.once.Do(func() { scopes.ok = landlockABI() >= scopeABI })
	return scopes.ok
}

// restrictThread scopes the calling thread: no new privileges, then a
// Landlock domain that handles no file or network access but scopes abstract
// Unix sockets and signals to itself. A variable so a test can make it fail.
var restrictThread = func() error {
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("no new privileges: %w", err)
	}
	attr := unix.LandlockRulesetAttr{Scoped: unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET | unix.LANDLOCK_SCOPE_SIGNAL}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return fmt.Errorf("landlock ruleset: %w", errno)
	}
	defer unix.Close(int(fd))
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0); errno != 0 {
		return fmt.Errorf("landlock restrict: %w", errno)
	}
	return nil
}

// Exec runs argv[0] in place of this process, scoped where the kernel allows:
// on one locked OS thread it restricts that thread, then execs from it, so
// the program inherits the domain. A kernel that cannot scope runs it
// unscoped (Scoped says so); one that can but refuses fails, and nothing
// runs. exec is syscall.Exec, or a stand-in for a test.
func Exec(path string, argv, env []string, exec func(string, []string, []string) error) error {
	runtime.LockOSThread()
	if Scoped() {
		if err := restrictThread(); err != nil {
			return fmt.Errorf("it could not be scoped: %w", err)
		}
	}
	return exec(path, argv, env)
}
