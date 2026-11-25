package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// runOniHands opens the Hands browser, lists its tabs, stops a task, says
// where Hands stands (the default), lists the task journal or prints the
// Hands ledger. Authorizing a tab, listing and picking a window, starting a
// task, resuming one, forgetting the Hands browser and deleting the journal
// are the user's own press in the Hyakki app or on Oni, never the command
// line.
func runOniHands(args []string, stdout, stderr io.Writer) int {
	rest, asJSON, _, ok := oniFlags(args, false)
	if !ok || len(rest) > 1 {
		return usageError(oniUsage)
	}
	verb := protocol.OniHandsStatus
	if len(rest) == 1 {
		verb = rest[0]
	}
	switch verb {
	case "log":
		return runOniHandsLog(asJSON, stdout, stderr)
	case protocol.OniHandsOpen, protocol.OniHandsTabs, protocol.OniHandsStop, protocol.OniHandsStatus, protocol.OniHandsTasks:
	case protocol.OniHandsAuthorize, protocol.OniHandsStart, protocol.OniHandsResume, protocol.OniHandsForget,
		protocol.OniHandsWindows, protocol.OniHandsSelect, protocol.OniHandsClear:
		fmt.Fprintln(stderr, i18n.T("Authorizing a tab or a window, starting Hands, resuming it, forgetting the Hands browser and deleting the task journal are done in the Hyakki app, under Settings, Oni, Hands, or on Oni's pill; the command line only opens the browser, lists its tabs, stops Hands, lists its tasks and says where it stands."))
		return 2
	default:
		return usageError(oniUsage)
	}
	var result protocol.OniHandsResult
	if err := callReflex(protocol.MOniHands, protocol.OniHandsParams{Action: verb}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if asJSON {
		return printOniJSON(stdout, result)
	}
	switch verb {
	case protocol.OniHandsTabs:
		for _, tab := range result.Tabs {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", tab.ID, tab.Origin, tab.Title)
		}
		if len(result.Tabs) == 0 {
			fmt.Fprintln(stdout, i18n.T("The Hands browser has no tab open."))
		}
		return 0
	case protocol.OniHandsTasks:
		printOniHandsTasks(stdout, result.Tasks)
		return 0
	case protocol.OniHandsStop:
		fmt.Fprintln(stdout, i18n.T("Hands is stopped."))
	}
	printOniHands(stdout, result)
	return 0
}

// printOniHands says where Hands stands and what it works with: the
// browser it uses, the window's app id, or whether the separate desktop
// can run; and what Oni said, if anything. It names an origin or an app
// id, never a page's or a window's text.
func printOniHands(w io.Writer, result protocol.OniHandsResult) {
	fmt.Fprintln(w, handsStateLine(result))
	switch {
	case len(result.Missing) > 0:
		fmt.Fprintln(w, i18n.Tf("The separate desktop needs %s, which this computer lacks. Hyakki does not install them.", strings.Join(result.Missing, ", ")))
	case result.Browser != "":
		fmt.Fprintln(w, i18n.Tf("Browser: %s", result.Browser))
	case handsOutsideBrowser(result):
	default:
		fmt.Fprintln(w, i18n.T("No Chromium-family browser was found. Hands does not install one."))
	}
	if result.Line != "" {
		fmt.Fprintln(w, result.Line)
	}
}

// handsOutsideBrowser says whether an answer is Hands at the app or
// desktop stage: it names no browser or tab, and a scope other than the
// tab, a window's app, a task, the separate desktop's lack, or a state
// the browser stage never has without a browser
func handsOutsideBrowser(r protocol.OniHandsResult) bool {
	if r.Browser != "" || r.Origin != "" {
		return false
	}
	return (r.Scope != "" && r.Scope != protocol.OniHandsScopeBrowser) || r.App != "" || r.Task != "" || len(r.Missing) > 0 ||
		r.State == protocol.OniHandsStateOpen || r.State == protocol.OniHandsStateAuthorized
}

// handsWhere says where a running task works
func handsWhere(r protocol.OniHandsResult) string {
	switch r.Scope {
	case protocol.OniHandsScopeWindow:
		return i18n.Tf("the window of %s", r.App)
	case protocol.OniHandsScopeNested, protocol.OniHandsScopeDesktop:
		return i18n.T("the separate desktop")
	}
	return r.Origin
}

// handsStateLine is one sentence on where Hands stands
func handsStateLine(result protocol.OniHandsResult) string {
	outside := handsOutsideBrowser(result)
	switch result.State {
	case protocol.OniHandsStateClosed:
		if len(result.Missing) > 0 {
			return i18n.T("The separate desktop cannot run.")
		}
		return i18n.T("The Hands browser is closed.")
	case protocol.OniHandsStateOpen:
		if result.Browser == "" {
			return i18n.T("No window is authorized.")
		}
		return i18n.T("The Hands browser is open, and no tab is authorized.")
	case protocol.OniHandsStateAuthorized:
		switch {
		case result.App != "":
			return i18n.Tf("Authorized window: %s. No task is running.", result.App)
		case outside:
			return i18n.T("The separate desktop is ready. No task is running.")
		}
		return i18n.Tf("Authorized tab: %s. No task is running.", result.Origin)
	case protocol.OniHandsStateWorking:
		return i18n.Tf("Working in %s, step %d of %d.", handsWhere(result), result.Step, result.Max)
	case protocol.OniHandsStateWaiting:
		return i18n.Tf("Waiting in %s at step %d of %d: press what Hands points at yourself.", handsWhere(result), result.Step, result.Max)
	case protocol.OniHandsStatePaused:
		return i18n.Tf("Paused in %s at step %d of %d (%s).", handsWhere(result), result.Step, result.Max, result.Pause)
	}
	return result.State
}

// printOniHandsTasks prints the task journal, newest first: the task, its
// state, where it worked, who started it, its step, when it was queued and
// ended, and why. A row holds no goal, title, address or typed word.
func printOniHandsTasks(w io.Writer, tasks []protocol.OniHandsTaskRow) {
	if len(tasks) == 0 {
		fmt.Fprintln(w, i18n.T("The task journal is empty."))
		return
	}
	dash := func(s string) string {
		if s == "" {
			return "-"
		}
		return s
	}
	for _, t := range tasks {
		ended := "-"
		if !t.Ended.IsZero() {
			ended = t.Ended.Local().Format(time.DateTime)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n", t.ID, t.State, t.Scope, dash(t.App), t.Origin, t.Step,
			t.At.Local().Format(time.DateTime), ended, dash(t.Reason))
	}
}

// runOniHandsLog prints the Hands ledger, newest first: when, the task,
// the step, the tool, what came of it and the rule's code behind it, and
// how long it took. A row holds no control name, page text or typed word.
func runOniHandsLog(asJSON bool, stdout, stderr io.Writer) int {
	var result protocol.OniHandsLogResult
	if err := callReflex(protocol.MOniHandsLog, protocol.Empty{}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if asJSON {
		return printOniJSON(stdout, result)
	}
	if len(result.Rows) == 0 {
		fmt.Fprintln(stdout, i18n.T("Hands has not acted yet."))
		return 0
	}
	for _, row := range result.Rows {
		reason := row.Reason
		if reason == "" {
			reason = "-"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%d\t%s\t%s\t%s\t%d ms\n", row.At.Local().Format(time.DateTime), row.Task, row.Step, row.Tool, row.Outcome, reason, row.Ms)
	}
	return 0
}
