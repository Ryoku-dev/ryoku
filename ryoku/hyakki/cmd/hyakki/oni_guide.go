package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/protocol"
)

// guideFlags are a guide verb's flags, taken out of args in any order:
// --json, and where allowed --preview FILE, --text Q, --forget KEY and
// --all; the rest are the words left.
type guideFlags struct {
	json    bool
	preview string
	text    string
	hasText bool
	forget  string
	all     bool
	rest    []string
}

func parseGuideFlags(args []string, allowed ...string) (guideFlags, bool) {
	var f guideFlags
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			f.rest = append(f.rest, arg)
			continue
		}
		if arg != "--json" && !slices.Contains(allowed, arg) {
			return f, false
		}
		value := func() (string, bool) {
			if index+1 >= len(args) || args[index+1] == "" {
				return "", false
			}
			index++
			return args[index], true
		}
		var ok bool
		switch arg {
		case "--json":
			f.json = true
			ok = true
		case "--all":
			f.all = true
			ok = true
		case "--preview":
			f.preview, ok = value()
		case "--text":
			f.text, ok = value()
			f.hasText = true
		case "--forget":
			f.forget, ok = value()
		}
		if !ok {
			return f, false
		}
	}
	return f, true
}

// runOniGuide starts a walkthrough of a goal in the perched window, or
// moves the running one on, throws the ring around its target again or
// stops it. A step whose picture needs a yes asks on stderr and reads one
// line, as oni ask does. A guide that ends with an offer to keep it says
// how to answer: oni keep or oni skip with the offer's id.
func runOniGuide(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f, ok := parseGuideFlags(args, "--preview")
	if !ok || len(f.rest) == 0 {
		return usageError(oniUsage)
	}
	params := protocol.OniGuideParams{Action: f.rest[0]}
	switch params.Action {
	case protocol.OniGuideStart:
		params.Goal = strings.TrimSpace(strings.Join(f.rest[1:], " "))
		if params.Goal == "" {
			return usageError(oniUsage)
		}
	case protocol.OniGuideNext, protocol.OniGuideStop, protocol.OniGuideCircle:
		if len(f.rest) != 1 {
			return usageError(oniUsage)
		}
	default:
		return usageError(oniUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	conn, cancel, err := dial(true)
	if err != nil {
		return reportOni(stderr, err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.OniAskResult
	if err := conn.Call(ctx, protocol.MOniGuide, params, &result); err != nil {
		return reportOni(stderr, err)
	}
	if code, sent := settleOniConsent(ctx, conn, stdin, stderr, f.preview, &result); !sent {
		return code
	}
	switch result.Status {
	case protocol.OniAskAnswered, protocol.OniAskEnded:
	case protocol.OniAskDeclined:
		fmt.Fprintln(stderr, i18n.T("Cancelled. Nothing was sent."))
		return 1
	default:
		return reportOni(stderr, errors.New(i18n.Tf("The daemon answered %q, which this CLI does not know.", result.Status)))
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	printOniGuide(stdout, stderr, result)
	return 0
}

// printOniGuide says the step, or how the guide ended and how to answer its
// keep offer
func printOniGuide(stdout, stderr io.Writer, result protocol.OniAskResult) {
	answer := result.Answer
	if result.Status == protocol.OniAskEnded {
		if answer == nil {
			fmt.Fprintln(stdout, i18n.T("No guide was running."))
		} else {
			fmt.Fprintln(stdout, answer.Text)
		}
		if offer := result.Keep; offer != nil {
			fmt.Fprintln(stdout, offer.Question)
			fmt.Fprintln(stdout, i18n.Tf("Run hyakki oni keep %s within %d seconds to keep it; hyakki oni skip %s, or no answer, keeps nothing.", offer.ID, (offer.ExpiresMs+999)/1000, offer.ID))
		}
		return
	}
	if answer == nil {
		return
	}
	if answer.Replay {
		fmt.Fprintln(stdout, i18n.Tf("Kept from before, step %d: %s", answer.Step, answer.Text))
	} else {
		fmt.Fprintln(stdout, i18n.Tf("Step %d: %s", answer.Step, answer.Text))
	}
	fmt.Fprintln(stderr, i18n.T("Click it in the window, or run hyakki oni guide next."))
	if answer.Replay {
		fmt.Fprintln(stderr, i18n.T("Replayed on this computer from a kept walkthrough; no picture was sent."))
	} else {
		fmt.Fprintln(stderr, oniRan(answer.Route, answer.Destination, answer.Path))
	}
}

// runOniKeep answers the offer to keep a finished guide: keep writes it to
// this computer's keeps, skip lets it go
func runOniKeep(verb string, args []string, stdout, stderr io.Writer) int {
	f, ok := parseGuideFlags(args)
	if !ok || len(f.rest) != 1 {
		return usageError(oniUsage)
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return reportOni(stderr, err)
	}
	defer cancel()
	defer conn.Close()
	ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	params := protocol.OniKeepParams{ID: f.rest[0]}
	if verb == "skip" {
		if err := conn.Call(ctx, protocol.MOniSkip, params, new(protocol.Empty)); err != nil {
			return reportOni(stderr, err)
		}
		if f.json {
			return printOniJSON(stdout, protocol.Empty{})
		}
		fmt.Fprintln(stdout, i18n.T("Not kept."))
		return 0
	}
	var result protocol.OniKeepResult
	if err := conn.Call(ctx, protocol.MOniKeep, params, &result); err != nil {
		return reportOni(stderr, err)
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	if result.Entries == 1 {
		fmt.Fprintln(stdout, i18n.T("Kept. 1 walkthrough is kept."))
	} else {
		fmt.Fprintln(stdout, i18n.Tf("Kept. %d walkthroughs are kept.", result.Entries))
	}
	return 0
}

// oniKept is one kept walkthrough as oni keeps lists it: its key, app and
// goal, how many steps and when it was kept and last used. A keep holds no
// picture or window title, so none is listed.
type oniKept struct {
	Key    string    `json:"key"`
	App    string    `json:"app"`
	Goal   string    `json:"goal"`
	Steps  int       `json:"steps"`
	Misses int       `json:"misses,omitempty"`
	Kept   time.Time `json:"kept"`
	Used   time.Time `json:"used"`
}

// oniKeepsResult is what oni keeps prints with --json: the walkthroughs
// kept, after any forgetting, and the key forgotten or that all were.
type oniKeepsResult struct {
	Keeps  []oniKept `json:"keeps"`
	Forgot string    `json:"forgot,omitempty"`
	All    bool      `json:"all,omitempty"`
	// MovedAside is where a keeps file that could not be read went
	MovedAside string `json:"movedAside,omitempty"`
	// HintsMovedAside is where a hint record that could not be read went
	HintsMovedAside string `json:"hintsMovedAside,omitempty"`
}

// runOniKeeps lists the walkthroughs Oni kept on this computer, the most
// recently used first, or forgets one by its key or all of them. --all also
// clears the record of hints offered from them. A keeps file or hint record
// that cannot be read is moved aside by --all, kept beside the new.
func runOniKeeps(args []string, stdout, stderr io.Writer) int {
	f, ok := parseGuideFlags(args, "--forget", "--all")
	if !ok || len(f.rest) != 0 || (f.all && f.forget != "") {
		return usageError(oniUsage)
	}
	store := keep.NewStore("")
	var result oniKeepsResult
	switch {
	case f.forget != "":
		if err := store.Forget(f.forget); err != nil {
			return reportOni(stderr, err)
		}
		result.Forgot = f.forget
	case f.all:
		if err := store.ForgetAll(); err != nil {
			var wire *protocol.Error
			if !errors.As(err, &wire) || wire.Reason != keep.ReasonDamaged {
				return reportOni(stderr, err)
			}
			if err := store.Reset(); err != nil {
				return reportOni(stderr, err)
			}
			result.MovedAside = store.DamagedPath()
		}
		hints := store.Hints()
		if err := hints.ForgetAll(); err != nil {
			var wire *protocol.Error
			if !errors.As(err, &wire) || wire.Reason != keep.ReasonHintsDamaged {
				return reportOni(stderr, err)
			}
			if err := hints.Reset(); err != nil {
				return reportOni(stderr, err)
			}
			result.HintsMovedAside = hints.DamagedPath()
		}
		result.All = true
	}
	entries, err := store.List()
	if err != nil {
		return reportOni(stderr, err)
	}
	result.Keeps = make([]oniKept, len(entries))
	for index, e := range entries {
		result.Keeps[index] = oniKept{Key: e.Key, App: e.App, Goal: e.Goal, Steps: len(e.Steps), Misses: e.Misses, Kept: e.Kept, Used: e.Used}
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	switch {
	case result.MovedAside != "":
		fmt.Fprintln(stdout, i18n.Tf("The kept walkthroughs could not be read; the file was moved aside to %s and none are kept now.", result.MovedAside))
	case result.All:
		fmt.Fprintln(stdout, i18n.T("Forgot every kept walkthrough."))
	case result.Forgot != "":
		fmt.Fprintln(stdout, i18n.Tf("Forgot %s.", result.Forgot))
	}
	if result.HintsMovedAside != "" {
		fmt.Fprintln(stdout, i18n.Tf("The record of hints offered could not be read; it was moved aside to %s and hints start over.", result.HintsMovedAside))
	}
	if result.Forgot != "" || result.All {
		return 0
	}
	if len(result.Keeps) == 0 {
		fmt.Fprintln(stdout, i18n.T("Nothing is kept."))
		return 0
	}
	for _, k := range result.Keeps {
		steps := i18n.T("1 step")
		if k.Steps != 1 {
			steps = i18n.Tf("%d steps", k.Steps)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", k.Key, steps, i18n.Tf("kept %s", k.Kept.Local().Format(time.DateOnly)), k.Goal)
	}
	return 0
}

// runOniPick arms the pick, the next click on the desktop choosing the spot
// Oni answers about with the question --text gives, and keeps its
// connection open until that click. When the click's picture needs a yes it
// asks on stderr and reads one line from stdin, as oni ask does: y or yes
// allows this picture once; anything else, end of input, an interrupt or
// the question running out of time sends nothing. The answer shows here and
// beside Oni. An interrupt before the click lets the pick go. cancel
// disarms it.
func runOniPick(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f, ok := parseGuideFlags(args, "--text", "--preview")
	if !ok || len(f.rest) > 1 {
		return usageError(oniUsage)
	}
	params := protocol.OniPickParams{Action: protocol.OniPickArm, Text: strings.TrimSpace(f.text)}
	if len(f.rest) == 1 {
		if f.rest[0] != protocol.OniPickCancel || f.hasText || f.preview != "" {
			return usageError(oniUsage)
		}
		params = protocol.OniPickParams{Action: protocol.OniPickCancel}
	}
	if f.hasText && params.Text == "" {
		return usageError(oniUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	conn, cancel, err := dial(true)
	if err != nil {
		return reportOni(stderr, err)
	}
	defer cancel()
	defer conn.Close()
	call, stopCall := context.WithTimeout(ctx, 30*time.Second)
	var result protocol.OniAskResult
	err = conn.Call(call, protocol.MOniPick, params, &result)
	stopCall()
	if err != nil {
		return reportOni(stderr, err)
	}
	if params.Action == protocol.OniPickCancel {
		if f.json {
			return printOniJSON(stdout, result)
		}
		fmt.Fprintln(stdout, i18n.T("Nothing waits for a click any more."))
		return 0
	}
	fmt.Fprintln(stderr, i18n.T("Click the spot to ask about. Oni waits for one click and answers here and beside itself."))
	result = protocol.OniAskResult{}
	if err := conn.Call(ctx, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickWait}, &result); err != nil {
		if ctx.Err() != nil {
			// a fresh context, so the interrupt still lets the pick go
			let, done := context.WithTimeout(context.Background(), 5*time.Second)
			_ = conn.Call(let, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickCancel}, nil)
			done()
			fmt.Fprintln(stderr, i18n.T("Cancelled. Nothing was sent."))
			return 1
		}
		return reportOni(stderr, err)
	}
	if code, sent := settleOniConsent(ctx, conn, stdin, stderr, f.preview, &result); !sent {
		return code
	}
	switch result.Status {
	case protocol.OniAskAnswered:
	case protocol.OniAskDeclined:
		fmt.Fprintln(stderr, i18n.T("Cancelled. Nothing was sent."))
		return 1
	case protocol.OniAskEnded:
		fmt.Fprintln(stderr, i18n.T("No spot was picked: the pick was cancelled, ran out of time or took a second click. Nothing was sent."))
		return 1
	default:
		return reportOni(stderr, errors.New(i18n.Tf("The daemon answered %q, which this CLI does not know.", result.Status)))
	}
	if result.Answer == nil {
		return reportOni(stderr, errors.New(i18n.T("The daemon answered without an answer.")))
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	printOniAnswer(stdout, stderr, *result.Answer)
	return 0
}
