package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/surface/launch"
	"ryoku-rashin/internal/protocol"
)

const oniUsage = "oni [status|characters] [--json] | oni show|hide|watch | oni perch WINDOW [--json] | oni detach [--json] | oni ask [--json] [--preview FILE] QUESTION | oni talk start|stop|cancel [--json] | oni speak stop [--json] | oni dictate start|stop|cancel [--mode exact|clean|polished] [--json] | oni voice-test [--seconds N | --ms N] [--mic NODE] [--json] | oni guide start GOAL|next|stop|circle [--json] [--preview FILE] | oni keep|skip ID [--json] | oni keeps [--forget KEY | --all] [--json] | oni pick [--text QUESTION] [--preview FILE] [--json] | oni pick cancel [--json] | oni reminders [--cancel ID | --all] [--json] | oni routines [--run [NAME] | --forget NAME | --all] [--json] | oni reflex log [--json] | oni hands [open|tabs|stop|status|tasks|log] [--json]"

// Selecting a character is the Hyakki app's alone, so the CLI only reads,
// shows or hides Oni, watches its frames, perches it on a window and asks
// about that window, walks through a task in it, answers and lists what
// Oni kept, lists or cancels its reminders, routines and quick-command
// ledger, and opens the Hands browser, lists its tabs, stops Hands and
// reads its ledger. Quick commands themselves are only ever spoken, and
// Hands is authorized, started and resumed only by the user's own press.
func init() {
	register(&Command{Name: "oni", Group: "oni", Usage: "[status|characters] [--json] | show|hide|watch | perch WINDOW | detach | ask QUESTION | talk start|stop|cancel | speak stop | dictate start|stop|cancel [--mode M] | voice-test [--seconds N] | guide start GOAL|next|stop|circle | keep|skip ID | keeps [--forget KEY|--all] | pick [--text Q]|cancel | reminders [--cancel ID|--all] | routines [--run [NAME]|--forget NAME|--all] | reflex log | hands [open|tabs|stop|status|tasks|log]", Summary: i18n.T("Show Oni's state and characters, put Oni on screen or away, watch its frames, perch it on a window and ask about it, walk through a task in it, keep or forget walkthroughs, pick a spot to ask about, talk, dictate and test the microphone, list and cancel reminders, run or forget routines, read the quick-command log, or open the Hands browser, list its tabs, stop Hands, list its tasks and read its log."), Run: runOni})
}

func runOni(args []string) int {
	verb := "status"
	if len(args) > 0 && args[0] != "--json" {
		verb, args = args[0], args[1:]
	}
	switch verb {
	case "perch":
		return runOniPerch(args, os.Stdout, os.Stderr)
	case "detach":
		return runOniDetach(args, os.Stdout, os.Stderr)
	case "ask":
		return runOniAsk(args, os.Stdin, os.Stdout, os.Stderr)
	case "talk":
		return runOniTalk(args, os.Stdout, os.Stderr)
	case "speak":
		return runOniSpeak(args, os.Stdout, os.Stderr)
	case "dictate":
		return runOniDictate(args, os.Stdout, os.Stderr)
	case "voice-test":
		return runOniVoiceTest(args, os.Stdout, os.Stderr)
	case "guide":
		return runOniGuide(args, os.Stdin, os.Stdout, os.Stderr)
	case "keep", "skip":
		return runOniKeep(verb, args, os.Stdout, os.Stderr)
	case "keeps":
		return runOniKeeps(args, os.Stdout, os.Stderr)
	case "pick":
		return runOniPick(args, os.Stdin, os.Stdout, os.Stderr)
	case "reminders":
		return runOniReminders(args, os.Stdout, os.Stderr)
	case "routines":
		return runOniRoutines(args, os.Stdout, os.Stderr)
	case "reflex":
		return runOniReflex(args, os.Stdout, os.Stderr)
	case "hands":
		return runOniHands(args, os.Stdout, os.Stderr)
	}
	switch verb {
	case "show", "hide", "watch":
		if len(args) != 0 {
			return usageError(oniUsage)
		}
	}
	switch verb {
	case "show":
		return showOni()
	case "hide":
		return hideOni()
	case "watch":
		return watchOniCommand()
	}
	asJSON := len(args) == 1 && args[0] == "--json"
	if (len(args) != 0 && !asJSON) || (verb != "status" && verb != "characters") {
		return usageError(oniUsage)
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	if verb == "characters" {
		var result protocol.OniCharactersResult
		if err := conn.Call(context.Background(), protocol.MOniCharacters, protocol.Empty{}, &result); err != nil {
			return report(err)
		}
		if asJSON {
			return printJSON(result)
		}
		printOniCharacters(os.Stdout, result.Active, result.Characters)
		return 0
	}
	var status protocol.OniStatus
	if err := conn.Call(context.Background(), protocol.MOniStatus, protocol.Empty{}, &status); err != nil {
		return report(err)
	}
	if asJSON {
		return printJSON(status)
	}
	printOniStatus(os.Stdout, status)
	return 0
}

// setOniHidden stores oni.hidden and reports whether Oni is on.
func setOniHidden(hidden bool) (bool, error) {
	conn, cancel, err := dial(true)
	if err != nil {
		return false, err
	}
	defer cancel()
	defer conn.Close()
	patch, err := json.Marshal(map[string]any{"oni": map[string]any{"hidden": hidden}})
	if err != nil {
		return false, err
	}
	var settings protocol.Settings
	if err := conn.Call(context.Background(), protocol.MSettingsSet, protocol.SettingsSetParams{Patch: patch}, &settings); err != nil {
		return false, err
	}
	return protocol.EffectiveOni(settings).Master, nil
}

// showOni puts Oni on screen: it stops hiding and starts the surface that
// draws it. Turning Oni itself on stays a choice made in Settings.
func showOni() int {
	active, err := setOniHidden(false)
	if err != nil {
		return report(err)
	}
	if !active {
		fmt.Fprintln(os.Stderr, i18n.T("Oni is off. Turn it on in Hyakki's Settings first."))
		return 1
	}
	if err := launch.Launch(context.Background()); err != nil {
		return report(err)
	}
	return 0
}

// hideOni takes Oni off screen without turning it off; the surface unmaps.
func hideOni() int {
	if _, err := setOniHidden(true); err != nil {
		return report(err)
	}
	return 0
}

func watchOniCommand() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	return watchOni(ctx, conn, os.Stdout)
}

// watchOni opens the oni event as an observer and prints every frame as one
// JSON line, the frames to start from first, until ctx ends or the daemon
// goes away.
func watchOni(ctx context.Context, conn *client.Conn, w io.Writer) int {
	var opened protocol.OniOpenResult
	if err := conn.Call(ctx, protocol.MOniOpen, protocol.OniOpenParams{Renderer: protocol.OniRendererObserver}, &opened); err != nil {
		return report(err)
	}
	for _, frame := range opened.Frames {
		fmt.Fprintln(w, string(frame))
	}
	for {
		select {
		case <-ctx.Done():
			return 0
		case event, ok := <-conn.Events():
			if !ok {
				if err := conn.Err(); err != nil && err != io.EOF {
					return report(err)
				}
				return 0
			}
			if event.Event == protocol.EvtOni {
				fmt.Fprintln(w, string(event.Data))
			}
		}
	}
}

func printOniStatus(w io.Writer, status protocol.OniStatus) {
	state := i18n.T("off")
	switch {
	case status.Active:
		state = i18n.T("on")
	case status.Master:
		state = i18n.T("off (Hyakki is off)")
	}
	fmt.Fprintf(w, "%s\t%s\n", state, status.Character)
	for _, c := range status.Capabilities.Features {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", c.ID, c.Stage, c.Reason)
	}
	printOniCharacters(w, status.Character, status.Characters)
}

func printOniCharacters(w io.Writer, active string, characters []protocol.OniCharacter) {
	for _, c := range characters {
		mark := " "
		if c.ID == active {
			mark = "*"
		}
		state := i18n.T("valid")
		switch {
		case !c.Valid:
			state = i18n.T("invalid")
		case c.Shipped:
			state = i18n.T("shipped")
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s\n", mark, c.ID, state, c.Name)
		for _, f := range c.Fallbacks {
			fmt.Fprintf(w, "    %s\n", i18n.Tf("fallback %s: %s, uses %s", f.Slot, f.Reason, f.Used))
		}
		for _, e := range c.Errors {
			fmt.Fprintf(w, "    %s\n", i18n.Tf("error %s", e))
		}
	}
}

// reportOni prints a refusal, with the perch reason code when the daemon
// gave one, so a script can tell a closed window from one with no geometry.
func reportOni(w io.Writer, err error) int {
	var wire *protocol.Error
	if errors.As(err, &wire) && wire.Reason != "" {
		fmt.Fprintf(w, "%s (%s)\n", wire.Message, wire.Reason)
		return 1
	}
	fmt.Fprintln(w, err)
	return 1
}

// oniFlags takes --json, and with preview --preview FILE, out of args.
func oniFlags(args []string, preview bool) (rest []string, asJSON bool, file string, ok bool) {
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			asJSON = true
		case "--preview":
			if !preview || index+1 >= len(args) || args[index+1] == "" {
				return nil, false, "", false
			}
			index++
			file = args[index]
		default:
			if strings.HasPrefix(args[index], "--") {
				return nil, false, "", false
			}
			rest = append(rest, args[index])
		}
	}
	return rest, asJSON, file, true
}

// runOniPerch perches Oni on a window named by the desktop's id for it.
func runOniPerch(args []string, stdout, stderr io.Writer) int {
	rest, asJSON, _, ok := oniFlags(args, false)
	if !ok || len(rest) != 1 {
		return usageError(oniUsage)
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return reportOni(stderr, err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.OniPerchResult
	if err := conn.Call(context.Background(), protocol.MOniPerch, protocol.OniPerchParams{Window: rest[0]}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if asJSON {
		return printOniJSON(stdout, result)
	}
	fmt.Fprintln(stdout, i18n.Tf("Perched on %s “%s” (window %s), %s corner.", result.App, result.Title, result.Window, result.Corner))
	if result.Box != nil {
		fmt.Fprintln(stdout, i18n.Tf("  on %s at %g,%g, %g × %g", result.Output, result.Box.X, result.Box.Y, result.Box.W, result.Box.H))
	}
	return 0
}

// runOniDetach lets go of the perched window; with nothing perched it is
// already done.
func runOniDetach(args []string, stdout, stderr io.Writer) int {
	rest, asJSON, _, ok := oniFlags(args, false)
	if !ok || len(rest) != 0 {
		return usageError(oniUsage)
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return reportOni(stderr, err)
	}
	defer cancel()
	defer conn.Close()
	if err := conn.Call(context.Background(), protocol.MOniDetach, protocol.OniDetachParams{}, new(protocol.Empty)); err != nil {
		return reportOni(stderr, err)
	}
	if asJSON {
		return printOniJSON(stdout, protocol.Empty{})
	}
	fmt.Fprintln(stdout, i18n.T("Oni let go of the window."))
	return 0
}

// runOniAsk asks about the perched window. When the daemon wants a yes
// first, it shows what would be sent and where on stderr and reads one line
// from stdin: y or yes allows this picture once, anything else, end of input,
// an interrupt or the question running out of time sends nothing.
func runOniAsk(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rest, asJSON, preview, ok := oniFlags(args, true)
	question := strings.TrimSpace(strings.Join(rest, " "))
	if !ok || question == "" {
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
	if err := conn.Call(ctx, protocol.MOniAsk, protocol.OniAskParams{Text: question}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if code, sent := settleOniConsent(ctx, conn, stdin, stderr, preview, &result); !sent {
		return code
	}
	switch result.Status {
	case protocol.OniAskAnswered:
	case protocol.OniAskDeclined:
		fmt.Fprintln(stderr, i18n.T("Cancelled. Nothing was sent."))
		return 1
	default:
		return reportOni(stderr, errors.New(i18n.Tf("The daemon answered %q, which this CLI does not know.", result.Status)))
	}
	if result.Answer == nil {
		return reportOni(stderr, errors.New(i18n.T("The daemon answered without an answer.")))
	}
	if asJSON {
		return printOniJSON(stdout, result)
	}
	printOniAnswer(stdout, stderr, *result.Answer)
	return 0
}

// printOniAnswer prints an answer about a window: its words and where it
// points on stdout, which model answered and how the picture was taken on
// stderr.
func printOniAnswer(stdout, stderr io.Writer, answer protocol.OniAnswer) {
	fmt.Fprintln(stdout, answer.Text)
	if answer.Point != nil {
		fmt.Fprintln(stdout, i18n.Tf("Pointed at %.0f%%, %.0f%% of the window, %.0f%% × %.0f%% of it.", answer.Point.X*100, answer.Point.Y*100, answer.Point.W*100, answer.Point.H*100))
	}
	fmt.Fprintln(stderr, oniRan(answer.Route, answer.Destination, answer.Path))
}

// settleOniConsent answers a result that waits for a yes: it shows what
// would be sent and where on stderr, writes the preview to file when one is
// named, reads one line from stdin and replaces result with what the
// answer came to. y or yes allows this picture once; anything else, end of
// input, an interrupt or the question running out of time sends nothing,
// and sent is false with the exit code. A result that waits for nothing is
// left as it is.
func settleOniConsent(ctx context.Context, conn *client.Conn, stdin io.Reader, stderr io.Writer, preview string, result *protocol.OniAskResult) (code int, sent bool) {
	if result.Status != protocol.OniAskConsent {
		return 0, true
	}
	consent := result.Consent
	if consent == nil {
		return reportOni(stderr, errors.New(i18n.T("The daemon asked for consent without saying what for; nothing was sent."))), false
	}
	if preview != "" {
		if err := os.WriteFile(preview, consent.Preview, 0o600); err != nil {
			fmt.Fprintln(stderr, i18n.Tf("The preview was not written: %v", err))
		}
	}
	clear(consent.Preview)
	printOniConsent(stderr, *consent)
	yes, expired := readOniConsent(ctx, stdin, time.Duration(consent.ExpiresMs)*time.Millisecond)
	answer := "no"
	if yes {
		answer = "yes"
	}
	*result = protocol.OniAskResult{}
	// a fresh context, so an interrupt still reaches the daemon as a no
	if err := conn.Call(context.Background(), protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: answer}, result); err != nil {
		var wire *protocol.Error
		switch {
		case expired || (errors.As(err, &wire) && wire.Code == protocol.CodeNotFound):
			fmt.Fprintln(stderr, i18n.T("The picture waited too long and was dropped. Nothing was sent."))
			return 1, false
		case !yes:
			fmt.Fprintln(stderr, i18n.T("Cancelled. Nothing was sent."))
			return 1, false
		}
		return reportOni(stderr, err), false
	}
	return 0, true
}

// printOniConsent says what would be sent and where, before anything is.
func printOniConsent(w io.Writer, consent protocol.OniConsent) {
	fmt.Fprintln(w, i18n.Tf("Oni took a picture of the perched window, %d × %d pixels, through %s.", consent.Width, consent.Height, consent.Path))
	if consent.Disclosure != "" {
		fmt.Fprintln(w, consent.Disclosure)
	}
	if consent.Destination == "local" {
		fmt.Fprintln(w, i18n.Tf("It stays on this computer: %s reads it here.", consent.Route))
	} else {
		fmt.Fprintln(w, i18n.Tf("It goes to %s and leaves this computer.", consent.Route))
	}
	if consent.Question != "" {
		fmt.Fprintln(w, consent.Question)
	}
	fmt.Fprint(w, i18n.Tf("Allow once? Type y and Enter to send; anything else sends nothing. The picture is dropped after %d seconds. ", (consent.ExpiresMs+999)/1000))
}

// readOniConsent reads one line: yes only for y or yes. It stops waiting
// when ctx ends or the question runs out of time, reporting which.
func readOniConsent(ctx context.Context, stdin io.Reader, ttl time.Duration) (yes, expired bool) {
	line := make(chan string, 1)
	go func() {
		text, _ := bufio.NewReader(stdin).ReadString('\n')
		line <- text
	}()
	timer := time.NewTimer(ttl)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, false
	case <-timer.C:
		return false, true
	case text := <-line:
		text = strings.ToLower(strings.TrimSpace(text))
		return text == "y" || text == "yes", false
	}
}

// oniRan says which model answered and how the picture was taken.
func oniRan(route, destination, path string) string {
	if destination == "local" {
		return i18n.Tf("Answered by %s on this computer; the picture was taken through %s.", route, path)
	}
	return i18n.Tf("Answered by %s, which received the picture; it was taken through %s.", route, path)
}

func printOniJSON(w io.Writer, value any) int {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return report(err)
	}
	return 0
}
