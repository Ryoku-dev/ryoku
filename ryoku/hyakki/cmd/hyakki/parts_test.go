package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// partsFixture is what components.list answers in these tests: a downloaded
// model, one in use, one Ryoku keeps, one not downloaded and a program.
var partsFixture = protocol.ComponentsResult{Components: []protocol.Component{
	{ID: "model:whisper-base.en-q5_1", Kind: protocol.ComponentModel, Title: "Speech model whisper-base.en-q5_1", State: protocol.ComponentPresent, Bytes: 59721011, Removable: true},
	{ID: "model:silero-v6.2.0", Kind: protocol.ComponentModel, Title: "Speech detection model silero-v6.2.0", State: protocol.ComponentPresent, Bytes: 885098, Removable: true,
		InUse: "The speech model Oni listens with uses it while voice is on. Turn voice off first."},
	{ID: "model:parakeet-tdt-0.6b-v2-q8_0", Kind: protocol.ComponentModel, Title: "Speech model parakeet", State: protocol.ComponentPresent, Bytes: 729574912,
		Why: "Ryoku's dictation keeps this model; Hyakki uses it where it is and never removes it."},
	{ID: "pack:kokoro", Kind: protocol.ComponentPack, Title: "Kokoro voice pack", State: protocol.ComponentAbsent, Removable: true},
	{ID: "program:tesseract", Kind: protocol.ComponentProgram, Title: "Tesseract text reader", State: protocol.ComponentPresent, Needs: []string{"tesseract"},
		Why: "Reads the words in a window so a guide can point at them. Your distribution installs it; Hyakki never installs or removes it."},
}}

// partsRun runs parts with the fixture listed and counts the app opens.
func partsRun(args ...string) (code int, out, errs string, opened int) {
	var o, e bytes.Buffer
	code = parts(args, &o, &e, func() (protocol.ComponentsResult, error) { return partsFixture, nil }, func() int { opened++; return 0 })
	return code, o.String(), e.String(), opened
}

func TestPartsCLIListsEveryPartWithItsFate(t *testing.T) {
	for _, args := range [][]string{nil, {"list"}} {
		code, out, errs, opened := partsRun(args...)
		if code != 0 || errs != "" || opened != 0 {
			t.Fatalf("%q: code %d, errs %q, opened %d", args, code, errs, opened)
		}
		want := "model:whisper-base.en-q5_1\tpresent\t59721011\tremovable\tSpeech model whisper-base.en-q5_1\n" +
			"model:silero-v6.2.0\tpresent\t885098\tin-use\tSpeech detection model silero-v6.2.0\n" +
			"model:parakeet-tdt-0.6b-v2-q8_0\tpresent\t729574912\tkept\tSpeech model parakeet\n" +
			"pack:kokoro\tabsent\t0\tremovable\tKokoro voice pack\n" +
			"program:tesseract\tpresent\t0\toptional\tTesseract text reader\n"
		if out != want {
			t.Fatalf("%q printed\n%s\nwant\n%s", args, out, want)
		}
	}
}

// Removing hands off to the app, which confirms it; a part in use, one Hyakki
// keeps, a program and an unknown id are refused with the reason, and
// nothing opens.
func TestPartsCLIRemoveRefusesOrHandsOffToTheApp(t *testing.T) {
	code, out, errs, opened := partsRun("remove", "model:whisper-base.en-q5_1")
	if code != 0 || opened != 1 || errs != "" || !strings.Contains(out, "Confirm removing Speech model whisper-base.en-q5_1 in Hyakki's Settings") {
		t.Fatalf("removable: code %d, opened %d, out %q, errs %q", code, opened, out, errs)
	}
	for id, why := range map[string]string{
		"model:silero-v6.2.0":             "Turn voice off first.",
		"model:parakeet-tdt-0.6b-v2-q8_0": "Ryoku's dictation keeps this model",
		"program:tesseract":               "Hyakki never installs or removes it.",
		"model:no-such":                   "Hyakki has no optional part called model:no-such.",
	} {
		code, out, errs, opened := partsRun("remove", id)
		if code != 1 || opened != 0 || out != "" || !strings.Contains(errs, why) {
			t.Fatalf("%s: code %d, opened %d, out %q, errs %q", id, code, opened, out, errs)
		}
	}
	code, out, _, opened = partsRun("remove", "pack:kokoro")
	if code != 0 || opened != 0 || !strings.Contains(out, "nothing to remove") {
		t.Fatalf("absent part: code %d, opened %d, out %q", code, opened, out)
	}
}

func TestPartsCLIRefusesBadArgumentsAndReportsTheDaemon(t *testing.T) {
	for _, args := range [][]string{{"remove"}, {"remove", "Model:X"}, {"remove", "a", "b"}, {"list", "extra"}, {"rm", "pack:kokoro"}} {
		code, out, errs, opened := partsRun(args...)
		if code != 2 || out != "" || opened != 0 || !strings.Contains(errs, "Usage: hyakki parts") {
			t.Fatalf("%q: code %d, out %q, errs %q, opened %d", args, code, out, errs, opened)
		}
	}
	var out, errs bytes.Buffer
	code := parts(nil, &out, &errs, func() (protocol.ComponentsResult, error) {
		return protocol.ComponentsResult{}, errors.New("the daemon is not running")
	}, func() int { t.Fatal("opened"); return 0 })
	if code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), "the daemon is not running") {
		t.Fatalf("daemon down: code %d, out %q, errs %q", code, out.String(), errs.String())
	}
}
