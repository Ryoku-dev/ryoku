package envpprof

import (
	"expvar"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/anacrolix/log"
)

var (
	pprofDir = filepath.Join(os.Getenv("HOME"), "pprof")
)

// Stop ends CPU profiling, waiting for writes to complete. If heap profiling is enabled, it also
// writes the heap profile to a file. Stop should be deferred from main if cpu or heap profiling
// are to be used through envpprof.
func Stop() {
	for _, profiler := range profilers {
		profiler.stop()
	}
}

func startHTTP(value string) {
	var l net.Listener
	if value == "" {
		for port := uint16(6061); port != 6060; port++ {
			var err error
			l, err = net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
			if err == nil {
				break
			}
		}
		if l == nil {
			log.Print("unable to create envpprof listener for http")
			return
		}
	} else {
		var addr string
		_, _, err := net.SplitHostPort(value)
		if err == nil {
			addr = value
		} else {
			addr = "localhost:" + value
		}
		l, err = net.Listen("tcp", addr)
		if err != nil {
			panic(err)
		}
	}
	log.Printf("(pid=%d) envpprof serving http://%s", os.Getpid(), l.Addr())
	go func() {
		defer l.Close()
		log.Printf("error serving http on envpprof listener: %s", http.Serve(l, nil))
	}()
}

func init() {
	expvar.Publish("numGoroutine", expvar.Func(func() interface{} { return runtime.NumGoroutine() }))

	envValue := os.Getenv("GOPPROF")
	if envValue == "" {
		return
	}
	for _, item := range strings.Split(envValue, ",") {
		equalsPos := strings.IndexByte(item, '=')
		var key, value string
		if equalsPos < 0 {
			key = item
		} else {
			key = item[:equalsPos]
			value = item[equalsPos+1:]
		}
		switch key {
		case "http":
			startHTTP(value)
		default:
			profiler, ok := profilers[key]
			if ok {
				profiler.start(key)
			} else {
				log.Printf("unexpected GOPPROF key %q", key)
			}
		}
	}
}
