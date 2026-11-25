package localmodels

import (
	"sort"
	"strconv"
	"strings"
)

func plainArgs(b binary, model presetModel, port int, keyFile, secret string) []string {
	flags := b.Flags
	args := []string{"--host", "127.0.0.1", "--port", strconv.Itoa(port)}
	if flags["--api-key-file"] {
		args = append(args, "--api-key-file", keyFile)
	} else if flags["--api-key"] {
		args = append(args, "--api-key", secret)
	}
	if flags["--model"] {
		args = append(args, "--model", model.Model)
	} else {
		args = append(args, "-m", model.Model)
	}
	if flags["--alias"] {
		args = append(args, "--alias", model.ID)
	} else if flags["-a"] {
		args = append(args, "-a", model.ID)
	}
	if model.Mmproj != "" && flags["--mmproj"] {
		args = append(args, "--mmproj", model.Mmproj)
	}
	if model.Context > 0 {
		if flags["--ctx-size"] {
			args = append(args, "--ctx-size", strconv.Itoa(model.Context))
		} else if flags["-c"] {
			args = append(args, "-c", strconv.Itoa(model.Context))
		}
	}
	if model.Device != "" {
		if flags["--device"] {
			args = append(args, "--device", model.Device)
		} else if flags["--dev"] {
			args = append(args, "--dev", model.Device)
		}
	}
	for _, flag := range []string{"--offline", "--no-webui", "--jinja"} {
		if flags[flag] {
			args = append(args, flag)
		}
	}
	keys := make([]string, 0, len(model.PlanArgs)+len(model.Args))
	for key := range model.PlanArgs {
		keys = append(keys, key)
	}
	for key := range model.Args {
		if !presetReserved[key] && presetKey(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := model.PlanArgs[key]
		if value == "" {
			value = model.Args[key]
		}
		if !presetValue(value) {
			continue
		}
		flag := "--" + key
		if key == "flash-attn" {
			// a bare -fa switch would swallow the value as a stray argument
			if !flashAttnAuto(flags) {
				continue
			}
			if !flags[flag] {
				flag = "-fa"
			}
		}
		if !flags[flag] {
			continue
		}
		if key == "fit" {
			if value == "on" {
				args = append(args, flag)
			}
			continue
		}
		if key == "n-gpu-layers" && value == "all" {
			value = "999"
		}
		if key == "flash-attn" && value == "" {
			value = "auto"
		}
		args = append(args, flag, value)
	}
	return args
}

func plainMissingCapabilities(b binary) string {
	var missing []string
	if !b.Flags["--api-key-file"] && !b.Flags["--api-key"] {
		missing = append(missing, "--api-key-file or --api-key")
	}
	if len(missing) > 0 {
		return strings.Join(missing, ", ")
	}
	return ""
}
