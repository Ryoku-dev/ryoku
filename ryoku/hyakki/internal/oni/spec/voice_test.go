package spec

import (
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestVoicePhasesDeriveCharacterStates(t *testing.T) {
	s := mustLoad(t)
	cases := []struct {
		phase, state string
		mark         bool
	}{
		{"idle", NoState, false},
		{"armed", NoState, true},
		{"listening", "listening", false},
		{"transcribing", "thinking", false},
		{"speaking", "speaking", false},
	}
	for _, c := range cases {
		state, mark, ok := s.VoiceState(c.phase)
		if !ok || state != c.state || mark != c.mark {
			t.Errorf("VoiceState(%q) = %q, %v, %v; want %q, %v, true", c.phase, state, mark, ok, c.state, c.mark)
		}
		if state != NoState {
			if _, ok := s.State(state); !ok {
				t.Errorf("phase %s derives %q, which has no shape row", c.phase, state)
			}
		}
	}
	if _, _, ok := s.VoiceState("whispering"); ok {
		t.Error("an unknown phase resolved")
	}
}

func TestVoiceTimingsHoldTheirOrder(t *testing.T) {
	s := mustLoad(t)
	v := s.Voice
	if !(v.Vad.MinSpeechMs < v.Vad.FirstPartialMs) {
		t.Error("speech must start before the first partial")
	}
	if !(v.Vad.MaxSegmentS < v.Press.MaxHoldS && v.Press.MaxHoldS < v.Press.OpenMicS) {
		t.Error("want maxSegment < maxHold < openMic")
	}
	if !(v.Wake.SessionS < v.Wake.ArmedMaxS) {
		t.Error("a wake session must end before wake disarms")
	}
	if got := v.Levels.MaxHz; got > 15 {
		t.Errorf("level frames at %g Hz, want at most 15", got)
	}
}

func TestEchoWindowGrowsWithTheLine(t *testing.T) {
	s := mustLoad(t)
	for _, c := range []struct {
		chars int
		ms    float64
	}{{0, 900}, {1, 970}, {10, 1600}, {-4, 900}} {
		if got := s.EchoMs(c.chars); got != c.ms {
			t.Errorf("EchoMs(%d) = %g, want %g", c.chars, got, c.ms)
		}
	}
}

// voiceLeaves lists every voice.* value path in the embedded spec.
func voiceLeaves(t *testing.T) map[string]string {
	t.Helper()
	flat, err := flattenJSON(embedded)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for p, v := range flat {
		if strings.HasPrefix(p, "voice.") {
			out[p] = v
		}
	}
	if len(out) < 30 {
		t.Fatalf("only %d voice values found", len(out))
	}
	return out
}

func setPath(root map[string]any, path string, v any) {
	i := strings.LastIndex(path, ".")
	node(root, path[:i])[path[i+1:]] = v
}

func TestEveryVoiceNumberRejectsANegative(t *testing.T) {
	leaves := voiceLeaves(t)
	for _, p := range sortedKeys(leaves) {
		if _, err := strconv.ParseFloat(leaves[p], 64); err != nil {
			continue
		}
		t.Run(p, func(t *testing.T) {
			_, err := Parse(edited(t, func(r map[string]any) { setPath(r, p, json.Number("-1")) }))
			if err == nil || !strings.Contains(err.Error(), p+": negative value") {
				t.Fatalf("negative %s: got %v", p, err)
			}
		})
	}
}

func TestEveryVoiceValueDriftBitesTheDoc(t *testing.T) {
	md, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	leaves := voiceLeaves(t)
	for _, p := range sortedKeys(leaves) {
		t.Run(p, func(t *testing.T) {
			var drifted any
			switch v := leaves[p]; v {
			case "true":
				drifted = false
			case "false":
				drifted = true
			default:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					drifted = json.Number(strconv.FormatFloat(f+1, 'g', -1, 64))
				} else {
					drifted = v + "x"
				}
			}
			js := edited(t, func(r map[string]any) { setPath(r, p, drifted) })
			diffs, err := parity(md, js)
			if err != nil {
				t.Fatal(err)
			}
			if len(diffs) != 1 || !strings.HasPrefix(diffs[0], p+":") {
				t.Fatalf("want one drift on %s, got %q", p, diffs)
			}
		})
	}
	t.Run("doc row dropped", func(t *testing.T) {
		diffs, err := parity([]byte(replaceOnce(t, string(md), "| pauseMs | 450 |\n", "")), embedded)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(diffs, []string{"voice.vad.pauseMs: missing from the doc"}) {
			t.Fatalf("got %q", diffs)
		}
	})
}

func TestVoiceLoadRejects(t *testing.T) {
	num := func(s string) json.Number { return json.Number(s) }
	set := func(path, v string) []byte {
		return edited(t, func(r map[string]any) { setPath(r, path, num(v)) })
	}
	phase := func(id, state string, mark bool) []byte {
		return edited(t, func(r map[string]any) {
			node(r, "voice.phases")[id] = map[string]any{"state": state, "micMark": mark}
		})
	}
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"missing voice", edited(t, func(r map[string]any) { delete(r, "voice") }), "voice: missing key"},
		{"unknown voice key", edited(t, func(r map[string]any) { node(r, "voice")["warmMicS"] = num("45") }), "voice.warmMicS: unknown key"},
		{"missing pause", edited(t, func(r map[string]any) { delete(node(r, "voice.vad"), "pauseMs") }), "voice.vad.pauseMs: missing key"},
		{"wrong sample rate", set("voice.vad.sampleRateHz", "48000"), "voice.vad.sampleRateHz: the speech engines read 16000 Hz only"},
		{"fractional sample rate", set("voice.vad.sampleRateHz", "16000.5"), "voice.vad.sampleRateHz: want an integer"},
		{"zero frame", set("voice.vad.frameMs", "0"), "voice.vad.frameMs: must be positive"},
		{"frame off the sample grid", set("voice.vad.frameMs", "0.03"), "voice.vad.frameMs: must be positive and a whole number of samples"},
		{"pause off the frame grid", set("voice.vad.pauseMs", "455"), "voice.vad.pauseMs: must be a whole number of voice.vad.frameMs"},
		{"segment off the frame grid", set("voice.vad.maxSegmentS", "9.005"), "voice.vad.maxSegmentS: must be a whole number"},
		{"pre-roll off the frame grid", set("voice.vad.preRollMs", "205"), "voice.vad.preRollMs: must be a whole number"},
		{"partial cadence off the frame grid", set("voice.vad.partialEveryMs", "305"), "voice.vad.partialEveryMs: must be a whole number"},
		{"min speech off the frame grid", set("voice.vad.minSpeechMs", "355"), "voice.vad.minSpeechMs: must be a whole number"},
		{"first partial off the frame grid", set("voice.vad.firstPartialMs", "655"), "voice.vad.firstPartialMs: must be a whole number"},
		{"silent speech threshold", set("voice.vad.speechRms", "0"), "voice.vad.speechRms: must lie in (0, 32768)"},
		{"unreachable speech threshold", set("voice.vad.speechRms", "40000"), "voice.vad.speechRms: must lie in (0, 32768)"},
		{"zero pause", set("voice.vad.pauseMs", "0"), "voice.vad.pauseMs: must be positive"},
		{"zero min speech", set("voice.vad.minSpeechMs", "0"), "voice.vad.minSpeechMs: must be positive"},
		{"zero partial cadence", set("voice.vad.partialEveryMs", "0"), "voice.vad.partialEveryMs: must be positive"},
		{"partial before speech starts", set("voice.vad.minSpeechMs", "700"), "voice.vad.firstPartialMs: speech must start before the first partial"},
		{"segment shorter than the first partial", set("voice.vad.maxSegmentS", "0.5"), "voice.vad.maxSegmentS: must outlast the first partial"},
		{"zero partial budget", set("voice.budgets.partialMs", "0"), "voice.budgets.partialMs: must be positive"},
		{"zero final budget", set("voice.budgets.finalS", "0"), "voice.budgets.finalS: must be positive"},
		{"partial budget past the final", set("voice.budgets.partialMs", "3000"), "voice.budgets.partialMs: must be shorter than the final budget"},
		{"zero tap drop", set("voice.press.tapDropMs", "0"), "voice.press.tapDropMs: must be positive"},
		{"hold shorter than a tap", set("voice.press.maxHoldS", "0.2"), "voice.press.maxHoldS: must outlast the tap drop"},
		{"hold shorter than a segment", set("voice.press.maxHoldS", "8"), "voice.press.maxHoldS: a hold must fit a whole segment"},
		{"open mic shorter than a hold", set("voice.press.openMicS", "20"), "voice.press.openMicS: must outlast a hold"},
		{"zero wake session", set("voice.wake.sessionS", "0"), "voice.wake.sessionS: must be positive"},
		{"wake session past open mic", set("voice.wake.sessionS", "200"), "voice.wake.sessionS: a wake session may not outlast open mic"},
		{"wake disarms inside a session", set("voice.wake.armedMaxS", "15"), "voice.wake.armedMaxS: must outlast a wake session"},
		{"zero ack gap", set("voice.turn.ackGapMs", "0"), "voice.turn.ackGapMs: must be positive"},
		{"zero echo base", set("voice.turn.echoBaseMs", "0"), "voice.turn.echoBaseMs: must be positive"},
		{"zero stt unload", set("voice.engines.sttIdleUnloadS", "0"), "voice.engines.sttIdleUnloadS: must be positive"},
		{"zero tts unload", set("voice.engines.ttsIdleUnloadS", "0"), "voice.engines.ttsIdleUnloadS: must be positive"},
		{"zero level rate", set("voice.levels.maxHz", "0"), "voice.levels.maxHz: must lie in (0, frames.tiersFps.active]"},
		{"level rate past the frame rate", set("voice.levels.maxHz", "61"), "voice.levels.maxHz: must lie in (0, frames.tiersFps.active]"},
		{"single repeat loop", set("voice.hygiene.loopRepeats", "1"), "voice.hygiene.loopRepeats: a loop needs at least 2 repeats"},
		{"fractional loop repeats", set("voice.hygiene.loopRepeats", "2.5"), "voice.hygiene.loopRepeats: want an integer"},
		{"zero loop words", set("voice.hygiene.loopMaxWords", "0"), "voice.hygiene.loopMaxWords: must be at least 1"},
		{"zero words per second", set("voice.hygiene.wordsPerS", "0"), "voice.hygiene.wordsPerS: must be positive"},
		{"fractional slack", set("voice.hygiene.slackWords", "3.5"), "voice.hygiene.slackWords: want an integer"},
		{"zero sentence words", set("voice.hygiene.sentenceMinWords", "0"), "voice.hygiene.sentenceMinWords: must be at least 1"},
		{"missing phase", edited(t, func(r map[string]any) { delete(node(r, "voice.phases"), "armed") }), "voice.phases: missing [armed]"},
		{"extra phase", phase("whispering", "listening", false), "voice.phases: missing [], unknown [whispering]"},
		{"unknown derived state", phase("transcribing", "pondering", false), `voice.phases.transcribing.state: unknown state "pondering"`},
		{"idle with a state", phase("idle", "ready", false), "voice.phases.idle: a quiet mic derives no state and shows no mic mark"},
		{"idle with the mic mark", phase("idle", "none", true), "voice.phases.idle: a quiet mic derives no state and shows no mic mark"},
		{"armed without the mic mark", phase("armed", "none", false), "voice.phases.armed: a live mic that changes no state must show the mic mark"},
		{"listening without a state or mark", phase("listening", "none", false), "voice.phases.listening: a live mic that changes no state must show the mic mark"},
		{"string mic mark", edited(t, func(r map[string]any) { node(r, "voice.phases.armed")["micMark"] = "yes" }), "voice.phases.armed.micMark: want a boolean"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := Parse(c.data)
			if err == nil {
				t.Fatalf("accepted; got spec version %d", s.Version)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not name %q", err, c.want)
			}
		})
	}
}
