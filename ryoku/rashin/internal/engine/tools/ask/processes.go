package ask

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// processesTool answers "what uses the most memory or CPU" with a table a
// small model reads right: per app (processes of one name summed), in human
// units, without the measuring tool itself. Small models misread raw ps and
// top columns and pipe them through sort in ways that scramble them.
type processesTool struct{}

func (processesTool) Name() string    { return tool.Processes }
func (processesTool) Kind() tool.Kind { return protocol.ToolRead }
func (processesTool) ReadOnly() bool  { return true }
func (processesTool) Spec(tool.Profile) llm.Tool {
	return askSpec(tool.Processes, "List the apps using the most memory or CPU right now, with totals.",
		`{"by":{"type":"string","enum":["memory","cpu"],"description":"memory or cpu"},"count":{"type":"integer","minimum":1,"maximum":25}}`, `["by"]`)
}
func (processesTool) Present(raw json.RawMessage) tool.Presentation {
	var args struct {
		By string `json:"by"`
	}
	_ = json.Unmarshal(raw, &args)
	if args.By == "cpu" {
		return tool.Presentation{Title: i18n.T("Checking what uses the processor")}
	}
	return tool.Presentation{Title: i18n.T("Checking what uses memory")}
}

func (processesTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	var args struct {
		By    string `json:"by"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return tool.Failed("Pass by as memory or cpu.")
	}
	if args.Count <= 0 || args.Count > 25 {
		args.Count = 10
	}
	if env.Exec == nil {
		return tool.Failed(i18n.T("Process listing is unavailable."))
	}
	switch args.By {
	case "cpu":
		out, err := runQuiet(ctx, env.Exec, "top", "-b", "-n", "2", "-d", "0.5", "-w", "512")
		if err != nil {
			return tool.Failed(err.Error())
		}
		return askResult(cpuTable(out, args.Count), env)
	case "memory", "":
		out, err := runQuiet(ctx, env.Exec, "ps", "-eo", "comm=,rss=")
		if err != nil {
			return tool.Failed(err.Error())
		}
		free, _ := runQuiet(ctx, env.Exec, "free", "-b")
		return askResult(memoryTable(out, free, args.Count), env)
	}
	return tool.Failed("Pass by as memory or cpu.")
}

func runQuiet(ctx context.Context, e exec.Executor, argv ...string) (string, error) {
	id, err := e.Start(ctx, exec.StartRequest{Argv: argv, Dir: paths.Home(), Timeout: 20 * time.Second, Keep: 512 * 1024}, nil)
	if err != nil {
		return "", err
	}
	defer e.Release(context.WithoutCancel(ctx), id)
	exit, err := e.Wait(ctx, id)
	if err != nil {
		_ = e.Kill(context.WithoutCancel(ctx), id)
		return "", err
	}
	if exit.Code != 0 {
		return "", fmt.Errorf("%s failed: %s", argv[0], strings.TrimSpace(exit.Output))
	}
	return exit.Output, nil
}

type usage struct {
	name  string
	value float64
	count int
}

func ranked(byName map[string]*usage, count int) []*usage {
	list := make([]*usage, 0, len(byName))
	for _, u := range byName {
		list = append(list, u)
	}
	sort.Slice(list, func(a, b int) bool {
		if list[a].value != list[b].value {
			return list[a].value > list[b].value
		}
		return list[a].name < list[b].name
	})
	if len(list) > count {
		list = list[:count]
	}
	return list
}

// memoryTable sums resident memory (ps rss, KiB) per command name.
func memoryTable(psOut, freeOut string, count int) string {
	byName := map[string]*usage{}
	for _, line := range strings.Split(psOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kib, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err != nil {
			continue
		}
		name := strings.Join(fields[:len(fields)-1], " ")
		if name == "ps" {
			continue
		}
		u := byName[name]
		if u == nil {
			u = &usage{name: name}
			byName[name] = u
		}
		u.value += kib * 1024
		u.count++
	}
	var b strings.Builder
	if total, available, ok := freeBytes(freeOut); ok {
		fmt.Fprintf(&b, "Memory: %s in use of %s, %s available.\n", bytesHuman(total-available), bytesHuman(total), bytesHuman(available))
	}
	list := ranked(byName, count)
	if len(list) > 0 {
		fmt.Fprintf(&b, "The app using the most memory is %s, with %s%s.\n", list[0].name, bytesHuman(list[0].value), plural(list[0].count))
	}
	b.WriteString("Apps by memory in use (all processes of one app added up):\n")
	for i, u := range list {
		fmt.Fprintf(&b, "%d. %s: %s%s\n", i+1, u.name, bytesHuman(u.value), plural(u.count))
	}
	return strings.TrimRight(b.String(), "\n")
}

func freeBytes(out string) (total, available float64, ok bool) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 7 && fields[0] == "Mem:" {
			t, e1 := strconv.ParseFloat(fields[1], 64)
			a, e2 := strconv.ParseFloat(fields[6], 64)
			return t, a, e1 == nil && e2 == nil
		}
	}
	return 0, 0, false
}

// cpuTable reads top's second frame (the first measures since boot) and
// sums %CPU per command name, leaving top itself out.
func cpuTable(out string, count int) string {
	frames := strings.Split(out, "\ntop - ")
	frame := frames[len(frames)-1]
	byName := map[string]*usage{}
	header := false
	for _, line := range strings.Split(frame, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "PID" {
			header = true
			continue
		}
		if !header || len(fields) < 12 {
			continue
		}
		cpu, err := strconv.ParseFloat(strings.ReplaceAll(fields[8], ",", "."), 64)
		if err != nil {
			continue
		}
		name := strings.Join(fields[11:], " ")
		if name == "top" {
			continue
		}
		u := byName[name]
		if u == nil {
			u = &usage{name: name}
			byName[name] = u
		}
		u.value += cpu
		u.count++
	}
	var b strings.Builder
	list := ranked(byName, count)
	if len(list) > 0 && list[0].value > 0 {
		fmt.Fprintf(&b, "The app using the most processor is %s, at %.0f%% of one core%s.\n", list[0].name, list[0].value, plural(list[0].count))
	}
	b.WriteString("Apps by processor use over the last half second (100% is one full core):\n")
	shown := 0
	for _, u := range list {
		if u.value <= 0 {
			continue
		}
		shown++
		fmt.Fprintf(&b, "%d. %s: %.0f%%%s\n", shown, u.name, u.value, plural(u.count))
	}
	if shown == 0 {
		b.WriteString("Nothing is using the processor noticeably right now.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func plural(count int) string {
	if count > 1 {
		return fmt.Sprintf(" (%d processes)", count)
	}
	return ""
}

func bytesHuman(v float64) string {
	const gib, mib = 1 << 30, 1 << 20
	if v >= gib {
		return fmt.Sprintf("%.1f GiB", v/gib)
	}
	return fmt.Sprintf("%.0f MiB", v/mib)
}
