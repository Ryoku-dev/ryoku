package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestAudioEventLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"Event 'change' on sink #53761", true},
		{"Event 'change' on sink-input #61164", true},
		{"Event 'change' on source #53990", true},
		{"Event 'change' on server", true},
		{"Event 'new' on sink 'alsa_output.pci-0000_63_00.6.analog-stereo'", true},
		{"Event 'remove' on sink #53761", true},
		{"Event 'new' on client #60912", false},
		{"Event 'change' on card #53756", false},
		{"Event 'remove' on client #60912", false},
		{"", false},
	}
	for _, c := range cases {
		if got := audioEventLine(c.line); got != c.want {
			t.Errorf("audioEventLine(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestParseAudioPercent(t *testing.T) {
	cases := []struct {
		out  string
		want float64
	}{
		{"Volume: front-left: 3977 /   6% / -73.02 dB,   front-right: 3977 /   6% / -73.02 dB\n        balance 0.00\n", 0.06},
		{"Volume: front-left: 28180 /  44% / -21.53 dB,   front-right: 28180 /  44% / -21.53 dB\n", 0.44},
		{"Volume: front-left: 98304 / 150% / +8.28 dB,   front-right: 98304 / 150% / +8.28 dB\n", 1.5},
		{"Volume: front-left: 65536 / 100% / 0.00 dB\n", 1.0},
		{"no percentages here\n", -1},
	}
	for _, c := range cases {
		if got := parseAudioPercent(c.out); fmt.Sprintf("%g", got) != fmt.Sprintf("%g", c.want) {
			t.Errorf("parseAudioPercent(%q) = %v, want %v", c.out, got, c.want)
		}
	}
}

func TestParseAudioMute(t *testing.T) {
	if !parseAudioMute("Mute: yes\n") {
		t.Fatal("Mute: yes must read as muted")
	}
	if parseAudioMute("Mute: no\n") {
		t.Fatal("Mute: no must read as unmuted")
	}
	if parseAudioMute("garbage") {
		t.Fatal("unparsable output must read as unmuted, not panic")
	}
}

func TestParseAudioDefaults(t *testing.T) {
	out := "Server name: /run/user/1000/pulse/native\n" +
		"Default Sink: alsa_output.pci-0000_63_00.6.analog-stereo\n" +
		"Default Source: alsa_input.pci-0000_63_00.6.analog-stereo\n" +
		"Cookie profile: ...\n"
	sink, source := parseAudioDefaults(out)
	if !strings.HasPrefix(sink, "alsa_output.") || !strings.HasPrefix(source, "alsa_input.") {
		t.Fatalf("defaults parsed wrong: %q %q", sink, source)
	}
}

func TestVolumePercentFormat(t *testing.T) {
	if got := volumePercent(0.45); got != "45.0%" {
		t.Fatalf("volumePercent(0.45) = %q", got)
	}
	if got := volumePercent(1.5); got != "150.0%" {
		t.Fatalf("volumePercent(1.5) = %q", got)
	}
}

func TestAudioStateReadsFrameFromServer(t *testing.T) {
	outs := map[string]string{
		"get-sink-volume @DEFAULT_SINK@":     "Volume: front-left: 28180 /  44% / -21.53 dB,   front-right: 28180 /  44% / -21.53 dB\n",
		"get-sink-mute @DEFAULT_SINK@":       "Mute: no\n",
		"get-source-volume @DEFAULT_SOURCE@": "Volume: front-left: 33285 /  52% / -17.00 dB,   front-right: 33285 /  52% / -17.00 dB\n",
		"get-source-mute @DEFAULT_SOURCE@":   "Mute: yes\n",
		"info":                               "Default Sink: alsa_output.pci-0000_63_00.6.analog-stereo\nDefault Source: alsa_input.pci-0000_63_00.6.analog-stereo\n",
	}
	old := runPactl
	runPactl = func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		if out, ok := outs[key]; ok {
			return out, nil
		}
		return "", fmt.Errorf("unexpected pactl %s", key)
	}
	t.Cleanup(func() { runPactl = old })

	frame := (&audioState{}).state()
	if frame.Volume != 0.44 || frame.Muted {
		t.Fatalf("sink half wrong: %+v", frame)
	}
	if frame.MicVolume != 0.52 || !frame.MicMuted {
		t.Fatalf("source half wrong: %+v", frame)
	}
	if frame.Sink == "" || frame.Source == "" {
		t.Fatalf("device names missing: %+v", frame)
	}
}

func TestAudioStateSurvivesDeadServer(t *testing.T) {
	old := runPactl
	runPactl = func(...string) (string, error) { return "", fmt.Errorf("no server") }
	t.Cleanup(func() { runPactl = old })

	frame := (&audioState{}).state()
	if frame.Volume != -1 || frame.MicVolume != -1 {
		t.Fatalf("unreadable server must report -1 levels, not zero: %+v", frame)
	}
}
