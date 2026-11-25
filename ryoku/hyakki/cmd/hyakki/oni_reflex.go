package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// reflexFlags are the flags of oni reminders and oni routines, taken out of
// args in any order: --json, --all, and where allowed --cancel ID, --forget
// NAME and --run with an optional NAME; the rest are the words left.
type reflexFlags struct {
	json   bool
	all    bool
	cancel string
	forget string
	run    bool
	name   string
	rest   []string
}

func parseReflexFlags(args []string, allowed ...string) (reflexFlags, bool) {
	var f reflexFlags
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			f.rest = append(f.rest, arg)
			continue
		}
		if arg != "--json" && !slices.Contains(allowed, arg) {
			return f, false
		}
		next := func() (string, bool) {
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "--") {
				return "", false
			}
			index++
			return args[index], true
		}
		switch arg {
		case "--json":
			f.json = true
		case "--all":
			f.all = true
		case "--cancel":
			value, ok := next()
			if !ok {
				return f, false
			}
			f.cancel = value
		case "--forget":
			value, ok := next()
			if !ok {
				return f, false
			}
			f.forget = value
		case "--run":
			// the name is optional: without one the most recently used runs
			f.run = true
			f.name, _ = next()
		}
	}
	return f, true
}

// callReflex makes one reflex call on a connection of its own
func callReflex(method string, params, result any) error {
	conn, cancel, err := dial(true)
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close()
	ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	return conn.Call(ctx, method, params, result)
}

// runOniReminders lists the reminders and timers Oni keeps, soonest first,
// or cancels the one --cancel names or every one with --all. Only a
// reminder's own words are listed, never what else was said.
func runOniReminders(args []string, stdout, stderr io.Writer) int {
	f, ok := parseReflexFlags(args, "--cancel", "--all")
	if !ok || len(f.rest) != 0 || (f.all && f.cancel != "") {
		return usageError(oniUsage)
	}
	params := protocol.OniRemindersParams{Action: protocol.OniRemindersList}
	switch {
	case f.cancel != "":
		if !protocol.ValidOniReminderID(f.cancel) {
			return usageError(oniUsage)
		}
		params = protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: f.cancel}
	case f.all:
		params.Action = protocol.OniRemindersCancelAll
	}
	var result protocol.OniRemindersResult
	if err := callReflex(protocol.MOniReminders, params, &result); err != nil {
		return reportOni(stderr, err)
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	switch params.Action {
	case protocol.OniRemindersCancel:
		fmt.Fprintln(stdout, i18n.Tf("Cancelled %s.", params.ID))
		return 0
	case protocol.OniRemindersCancelAll:
		fmt.Fprintln(stdout, i18n.T("Cancelled every reminder and timer."))
		return 0
	}
	if len(result.Reminders) == 0 {
		fmt.Fprintln(stdout, i18n.T("No reminder or timer is set."))
		return 0
	}
	for _, r := range result.Reminders {
		kind := i18n.T("reminder")
		if r.Kind == protocol.OniReminderKindTimer {
			kind = i18n.T("timer")
		}
		line := fmt.Sprintf("%s\t%s\t%s", r.ID, kind, r.Due.Local().Format("Mon 2 Jan 15:04"))
		if r.Text != "" {
			line += "\t" + r.Text
		}
		if r.Late {
			line += "\t" + i18n.T("(late)")
		}
		fmt.Fprintln(stdout, line)
	}
	if result.NoWake {
		fmt.Fprintln(stderr, i18n.T("This computer cannot start Oni for a reminder on its own, so Oni stays running while one is set."))
	}
	return 0
}

// runOniRoutines lists the routines Oni keeps, the most recently used
// first, replays one with --run (the most recently used when no name is
// given), or forgets one with --forget or every one with --all. A routine
// is listed by its name and length; its steps stay in the store.
func runOniRoutines(args []string, stdout, stderr io.Writer) int {
	f, ok := parseReflexFlags(args, "--run", "--forget", "--all")
	chosen := 0
	for _, on := range []bool{f.run, f.forget != "", f.all} {
		if on {
			chosen++
		}
	}
	if !ok || len(f.rest) != 0 || chosen > 1 {
		return usageError(oniUsage)
	}
	params := protocol.OniRoutinesParams{Action: protocol.OniRoutinesList}
	switch {
	case f.run:
		if f.name != "" && !protocol.ValidOniRoutineName(f.name) {
			return usageError(oniUsage)
		}
		params = protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: f.name}
	case f.forget != "":
		if !protocol.ValidOniRoutineName(f.forget) {
			return usageError(oniUsage)
		}
		params = protocol.OniRoutinesParams{Action: protocol.OniRoutinesForget, Name: f.forget}
	case f.all:
		params.Action = protocol.OniRoutinesForgetAll
	}
	var result protocol.OniRoutinesResult
	if err := callReflex(protocol.MOniRoutines, params, &result); err != nil {
		return reportOni(stderr, err)
	}
	if params.Action == protocol.OniRoutinesRun && !result.Ran {
		return reportOni(stderr, errors.New(i18n.T("No routine was run.")))
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	switch params.Action {
	case protocol.OniRoutinesRun:
		name := params.Name
		if name == "" && len(result.Routines) > 0 {
			name = result.Routines[0].Name
		}
		fmt.Fprintln(stdout, i18n.Tf("Ran %s.", name))
		return 0
	case protocol.OniRoutinesForget:
		fmt.Fprintln(stdout, i18n.Tf("Forgot %s.", params.Name))
		return 0
	case protocol.OniRoutinesForgetAll:
		fmt.Fprintln(stdout, i18n.T("Forgot every routine."))
		return 0
	}
	if len(result.Routines) == 0 {
		fmt.Fprintln(stdout, i18n.T("No routine is kept."))
		return 0
	}
	for _, r := range result.Routines {
		steps := i18n.T("1 step")
		if r.Steps != 1 {
			steps = i18n.Tf("%d steps", r.Steps)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", r.Name, steps, i18n.Tf("saved %s", r.Saved.Local().Format(time.DateOnly)), i18n.Tf("used %s", r.Used.Local().Format(time.DateOnly)))
	}
	return 0
}

// runOniReflex prints the quick-command ledger, newest first: when, which
// command, the route that named it, how long deciding took, how sure it
// was and what came of it. A row holds no words.
func runOniReflex(args []string, stdout, stderr io.Writer) int {
	f, ok := parseReflexFlags(args)
	if !ok || len(f.rest) != 1 || f.rest[0] != "log" {
		return usageError(oniUsage)
	}
	var result protocol.OniReflexLogResult
	if err := callReflex(protocol.MOniReflexLog, protocol.Empty{}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	if len(result.Rows) == 0 {
		fmt.Fprintln(stdout, i18n.T("No quick command has run yet."))
		return 0
	}
	for _, row := range result.Rows {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%d ms\t%.0f%%\t%s\n", row.At.Local().Format(time.DateTime), row.Intent, row.Route, row.Ms, row.Confidence*100, row.Outcome)
	}
	return 0
}
