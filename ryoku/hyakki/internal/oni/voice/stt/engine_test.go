//go:build sttengine

// The real-engine check: run with -tags sttengine and a PATH directory
// that holds whisper-server, ggml-base.en-q5_1.bin and speech.wav (16 kHz
// mono s16 of "Open the notes app and start a timer for five minutes.").
// ryoku-dictate and Ryoku's Parakeet 110M are used when installed. Files
// in, loopback HTTP and stdout out: no device.
package stt

import (
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func engineClip(t *testing.T, dir string) []int16 {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "speech.wav"))
	if err != nil {
		t.Fatal(err)
	}
	// find the data chunk
	for i := 12; i+8 <= len(data); {
		id, size := string(data[i:i+4]), int(binary.LittleEndian.Uint32(data[i+4:]))
		if id == "data" {
			raw := data[i+8 : min(len(data), i+8+size)]
			pcm := make([]int16, len(raw)/2)
			for j := range pcm {
				pcm[j] = int16(binary.LittleEndian.Uint16(raw[2*j:]))
			}
			return pcm
		}
		i += 8 + size + size%2
	}
	t.Fatal("speech.wav has no data chunk")
	return nil
}

func heard(t *testing.T, text string) {
	t.Helper()
	low := strings.ToLower(text)
	if !strings.Contains(low, "note") || !strings.Contains(low, "timer") {
		t.Fatalf("transcript %q misses the clip's words", text)
	}
}

func TestRealWhisperServer(t *testing.T) {
	bin, err := exec.LookPath("whisper-server")
	if err != nil {
		t.Skip("no whisper-server on PATH")
	}
	dir := filepath.Dir(bin)
	m, _ := FindModel("whisper-base.en-q5_1")
	l := specLimits(t)
	w, err := NewWhisper(WhisperConfig{Binary: bin, Model: filepath.Join(dir, m.File), SHA256: m.SHA256, License: m.License, Character: "Oni", Limits: l})
	if err != nil {
		t.Fatal(err)
	}
	pcm := engineClip(t, dir)
	if err := w.Warm(); err != nil {
		t.Fatal(err)
	}
	s := NewStream(w, l, "en", []string{"Mara"})
	part, ok, err := s.Partial(context.Background(), pcm[:min(len(pcm), int(1.2*SampleRate))])
	if err != nil || !ok {
		t.Fatalf("partial ok=%v err=%v", ok, err)
	}
	t.Logf("partial %q in %v (includes the cold load)", part.Text, part.Took)
	fin, err := s.Final(context.Background(), pcm)
	if err != nil {
		t.Fatal(err)
	}
	heard(t, fin.Text)
	if fin.Took > l.Final {
		t.Fatalf("final took %v, budget %v", fin.Took, l.Final)
	}
	t.Logf("final %q in %v", fin.Text, fin.Took)
	w.mu.Lock()
	pid, port := w.srv.proc.Pid(), w.srv.port
	w.mu.Unlock()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if procListening(pid, port) {
		t.Fatal("the engine still listens after Close")
	}
	if w.LastUnload() <= 0 {
		t.Fatal("no unload time recorded")
	}
	t.Logf("unload %v", w.LastUnload())
	if w.LastUnload() >= stopGrace {
		t.Fatalf("unload needed SIGKILL: %v", w.LastUnload())
	}
}

func TestRealRyokuDictate(t *testing.T) {
	bin, err := exec.LookPath("whisper-server")
	if err != nil {
		t.Skip("no clip directory on PATH")
	}
	if _, err := exec.LookPath("ryoku-dictate"); err != nil {
		t.Skip("ryoku-dictate is not installed")
	}
	m, _ := FindModel("parakeet-tdt_ctc-110m-q8_0")
	path, ok := Locate(m)
	if !ok {
		t.Skip("Parakeet 110M is not installed")
	}
	d, err := NewDictate(DictateConfig{Model: path, SHA256: m.SHA256, License: m.License, Language: "en", Limits: specLimits(t)})
	if err != nil {
		t.Fatal(err)
	}
	res, err := d.Transcribe(context.Background(), engineClip(t, filepath.Dir(bin)), Hint{Pass: Final})
	if err != nil {
		t.Fatal(err)
	}
	heard(t, res.Text)
	t.Logf("final %q in %v (one child: load and decode)", res.Text, res.Took)
}
