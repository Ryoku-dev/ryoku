package main

// gpu.go: the `ryoku-hub gpu ...` subcommand. The Hub GPU page calls these to read
// the passthrough capability verdict and (later phases) to switch graphics mode and
// enable/disable the passthrough stack.

import "fmt"

func runGpu(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu needs a subcommand: caps|mode|mux|apply|tune|hook|vm")
	}
	switch args[0] {
	case "caps":
		report, err := detectCapability()
		if err != nil {
			return err
		}
		return printJSON(report)
	case "mode":
		return runGpuMode(args[1:])
	case "mux":
		return runGpuMux(args[1:])
	case "apply":
		return runGpuApply(args[1:])
	case "tune":
		return runGpuTune(args[1:])
	case "hook":
		return runGpuHook(args[1:])
	case "vm":
		return runGpuVM(args[1:])
	default:
		return fmt.Errorf("unknown gpu subcommand: %s", args[0])
	}
}
