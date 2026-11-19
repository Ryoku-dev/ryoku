package envpprof

import (
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"

	"github.com/anacrolix/log"
)

type profiler interface {
	start(name string)
	stop()
}

var profilers = map[string]profiler{
	"trace": &continuousWriter{
		profileName: "trace",
		startWriter: trace.Start,
		stopFunc:    trace.Stop,
	},
	"mutex": &pprofWrite{
		startFunc: func() {
			// Taken from Safe Rate at
			// https://github.com/DataDog/go-profiler-notes/blob/main/guide/README.md#go-profilers.
			runtime.SetMutexProfileFraction(100)
		},
	},
	"heap": &pprofWrite{},
	"block": &pprofWrite{
		startFunc: func() {
			// Taken from Safe Rate at
			// https://github.com/DataDog/go-profiler-notes/blob/main/guide/README.md#go-profilers.
			runtime.SetBlockProfileRate(10000)
		},
	},
	"cpu": &continuousWriter{
		profileName: "cpu",
		startWriter: pprof.StartCPUProfile,
		stopFunc:    pprof.StopCPUProfile,
	},
}

type continuousWriter struct {
	profileName string
	startWriter func(io.Writer) error
	stopFunc    func()
	// If not nil, the profiler is active.
	file *os.File
}

func (me *continuousWriter) start(name string) {
	if me.file != nil {
		return
	}
	me.file = newPprofFileOrLog(name)
	if me.file == nil {
		return
	}
	err := me.startWriter(me.file)
	if err != nil {
		log.Printf("error starting %v profiling: %v", name, err)
		me.file.Close()
		me.file = nil
		return
	}
	log.Printf("%v profiling to %q", name, me.file.Name())
}

func (me *continuousWriter) stop() {
	if me.file == nil {
		return
	}
	me.stopFunc()
	me.file.Close()
	logWroteProfile(me.file, me.profileName)
	me.file = nil
}

type pprofWrite struct {
	// This is not "" if the profile type is enabled (and requested).
	pprofProfileName string
	startFunc        func()
}

func (me *pprofWrite) start(name string) {
	me.pprofProfileName = name
	if me.startFunc != nil {
		me.startFunc()
	}
}

func (me *pprofWrite) stop() {
	if me.pprofProfileName == "" {
		return
	}
	f := newPprofFileOrLog(me.pprofProfileName)
	if f == nil {
		return
	}
	defer f.Close()
	pprof.Lookup(me.pprofProfileName).WriteTo(f, 0)
	logWroteProfile(f, me.pprofProfileName)
}
