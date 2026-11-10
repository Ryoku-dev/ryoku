package desktop

import "os/exec"

// returns when the program ends, so the caller reaps it
func Launch(name string, args []string) error {
	return exec.Command(name, args...).Run()
}
