package envpprof

import (
	"os"

	"github.com/anacrolix/log"
)

func logWroteProfile(f *os.File, profile string) {
	log.Printf("wrote %v profile to %q", profile, f.Name())
}

func newPprofFileOrLog(profile string) (f *os.File) {
	os.Mkdir(pprofDir, 0750)
	f, err := os.CreateTemp(pprofDir, profile)
	if err != nil {
		log.Printf("error creating %v pprof file: %v", profile, err)
	}
	return
}
