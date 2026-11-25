package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// voiceCallTimeout bounds a voice call: a dictation's stop returns once its
// text is typed, and a mic test listens up to five seconds.
const voiceCallTimeout = 30 * time.Second

// voiceFlags takes --json, and where allowed --mode M, --seconds N, --ms N
// and --mic NODE, out of args in any order; the rest are the words left.
type voiceFlags struct {
	json bool
	mode string
	ms   int
	mic  string
	rest []string
}

func parseVoiceFlags(args []string, allowed ...string) (voiceFlags, bool) {
	var f voiceFlags
	value := func(index *int) (string, bool) {
		if *index+1 >= len(args) || args[*index+1] == "" {
			return "", false
		}
		*index++
		return args[*index], true
	}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			f.rest = append(f.rest, arg)
			continue
		}
		name, inline, hasInline := strings.Cut(arg, "=")
		if name != "--json" && !slices.Contains(allowed, name) {
			return f, false
		}
		get := func() (string, bool) {
			if hasInline {
				return inline, inline != ""
			}
			return value(&index)
		}
		switch name {
		case "--json":
			if hasInline {
				return f, false
			}
			f.json = true
		case "--mode":
			v, ok := get()
			if !ok || !slices.Contains(protocol.OniDictateModes, v) {
				return f, false
			}
			f.mode = v
		case "--seconds":
			v, ok := get()
			s, err := strconv.ParseFloat(v, 64)
			if !ok || err != nil || math.IsNaN(s) {
				return f, false
			}
			f.ms = int(math.Round(s * 1000))
			if f.ms < protocol.OniVoiceTestMinMs || f.ms > protocol.OniVoiceTestMaxMs {
				return f, false
			}
		case "--ms":
			v, ok := get()
			n, err := strconv.Atoi(v)
			if !ok || err != nil || n < protocol.OniVoiceTestMinMs || n > protocol.OniVoiceTestMaxMs {
				return f, false
			}
			f.ms = n
		case "--mic":
			v, ok := get()
			if !ok || !protocol.ValidOniMicNode(v) {
				return f, false
			}
			f.mic = v
		}
	}
	return f, true
}

// voiceCall dials the daemon, starting it only for a call that opens the
// microphone, and makes one voice call.
func voiceCall(start bool, method string, params, result any) error {
	conn, cancel, err := dial(start)
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close()
	ctx, stop := context.WithTimeout(context.Background(), voiceCallTimeout)
	defer stop()
	return conn.Call(ctx, method, params, result)
}

// runOniTalk starts, stops or cancels one talk press, as a compositor bind
// does on press and release.
func runOniTalk(args []string, stdout, stderr io.Writer) int {
	f, ok := parseVoiceFlags(args)
	if !ok || len(f.rest) != 1 || !slices.Contains(protocol.OniVoiceActions, f.rest[0]) {
		return usageError(oniUsage)
	}
	action := f.rest[0]
	var result protocol.OniVoiceResult
	if err := voiceCall(action == protocol.OniVoiceStart, protocol.MOniTalk, protocol.OniTalkParams{Action: action}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	fmt.Fprintln(stdout, voicePhaseText(result.Phase))
	return 0
}

// runOniSpeak stops what Oni is saying.
func runOniSpeak(args []string, stdout, stderr io.Writer) int {
	f, ok := parseVoiceFlags(args)
	if !ok || len(f.rest) != 1 || f.rest[0] != protocol.OniVoiceStop {
		return usageError(oniUsage)
	}
	var result protocol.OniVoiceResult
	if err := voiceCall(false, protocol.MOniSpeak, protocol.OniSpeakParams{Action: protocol.OniVoiceStop}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	fmt.Fprintln(stdout, voicePhaseText(result.Phase))
	return 0
}

// runOniDictate starts, stops or cancels one dictation into the window
// focused at start. Stop returns once the text is typed; text that could not
// be typed is printed so it can be copied.
func runOniDictate(args []string, stdout, stderr io.Writer) int {
	f, ok := parseVoiceFlags(args, "--mode")
	if !ok || len(f.rest) != 1 || !slices.Contains(protocol.OniVoiceActions, f.rest[0]) {
		return usageError(oniUsage)
	}
	action := f.rest[0]
	if f.mode != "" && action != protocol.OniVoiceStart {
		return usageError(oniUsage)
	}
	var result protocol.OniDictateResult
	if err := voiceCall(action == protocol.OniVoiceStart, protocol.MOniDictate, protocol.OniDictateParams{Action: action, Mode: f.mode}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	printOniDictate(stdout, action, result)
	return 0
}

func printOniDictate(w io.Writer, action string, r protocol.OniDictateResult) {
	if action != protocol.OniVoiceStop || r.Mode == "" {
		fmt.Fprintln(w, voicePhaseText(r.Phase))
		return
	}
	fmt.Fprintln(w, i18n.Tf("Typed %d characters (%s).", r.Landed, r.Mode))
	if r.Fallback != "" {
		fmt.Fprintln(w, i18n.Tf("Polishing fell back to clean: %s", r.Fallback))
	}
	if r.Refused != "" {
		fmt.Fprintln(w, i18n.Tf("Stopped typing: the window was %s.", r.Refused))
	}
	if r.Unsure > 0 {
		fmt.Fprintln(w, i18n.Tf("%d characters may or may not have been typed.", r.Unsure))
	}
	if r.Rest != "" {
		fmt.Fprintln(w, i18n.T("Not typed, to copy:"))
		fmt.Fprintln(w, r.Rest)
	}
}

// runOniVoiceTest listens to the microphone for Settings' meter and prints
// the levels heard and the sources there are. Nothing is recorded.
func runOniVoiceTest(args []string, stdout, stderr io.Writer) int {
	f, ok := parseVoiceFlags(args, "--seconds", "--ms", "--mic")
	if !ok || len(f.rest) != 0 {
		return usageError(oniUsage)
	}
	var result protocol.OniVoiceTestResult
	if err := voiceCall(true, protocol.MOniVoiceTest, protocol.OniVoiceTestParams{Mic: f.mic, Ms: f.ms}, &result); err != nil {
		return reportOni(stderr, err)
	}
	if f.json {
		return printOniJSON(stdout, result)
	}
	mic := result.Mic
	if mic == "" {
		mic = i18n.T("the default microphone")
	}
	peak := 0.0
	var meter strings.Builder
	for _, l := range result.Levels {
		peak = max(peak, l)
		meter.WriteRune(levelRune(l))
	}
	fmt.Fprintln(stdout, i18n.Tf("Listened to %s: peak %d%%", mic, int(math.Round(peak*100))))
	if meter.Len() > 0 {
		fmt.Fprintln(stdout, "  "+meter.String())
	}
	if len(result.Sources) > 0 {
		fmt.Fprintln(stdout, i18n.T("Microphones:"))
		for _, s := range result.Sources {
			fmt.Fprintf(stdout, "  %s  %s\n", s.Name, s.Desc)
		}
	}
	return 0
}

var levelRunes = []rune("▁▂▃▄▅▆▇█")

func levelRune(l float64) rune {
	i := int(math.Round(min(max(l, 0), 1) * float64(len(levelRunes)-1)))
	return levelRunes[i]
}

// voicePhaseText says what the microphone is doing.
func voicePhaseText(phase string) string {
	switch phase {
	case feature.VoiceIdle:
		return i18n.T("The microphone is closed.")
	case feature.VoiceArmed:
		return i18n.T("The microphone stays open for the wake phrase or open mic.")
	case feature.VoiceListening:
		return i18n.T("Oni is listening.")
	case feature.VoiceTranscribing:
		return i18n.T("Oni is working out what you said.")
	case feature.VoiceSpeaking:
		return i18n.T("Oni is speaking.")
	}
	return i18n.Tf("Voice: %s", phase)
}
