package tts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ryoku-rashin/internal/oni/voice/clips"
	"ryoku-rashin/internal/paths"
)

// KokoroBinary is sherpa-onnx's offline speech program.
const KokoroBinary = "sherpa-onnx-offline-tts"

// kokoroThreads is how many threads one render uses.
const kokoroThreads = 4

// Files a Kokoro pack folder holds. Lexicons (lexicon*.txt) are optional.
const (
	kokoroModel  = "model.onnx"
	kokoroVoices = "voices.bin"
	kokoroTokens = "tokens.txt"
	kokoroData   = "espeak-ng-data"
)

// KokoroOptions configure the optional neural voice.
type KokoroOptions struct {
	Binary  string  // "" finds KokoroBinary on PATH
	Dir     string  // the pack; "" is KokoroDir()
	Speaker int     // profile voice.speaker
	Speed   float64 // profile voice.speed; 0 is 1
	Run     Runner  // nil is Exec
}

// KokoroDir is where the optional Kokoro pack is installed:
// $XDG_DATA_HOME/hyakki/oni/voice/kokoro. Removing the folder removes the
// voice.
func KokoroDir() string { return filepath.Join(paths.DataHome(), "hyakki", "oni", "voice", "kokoro") }

// KokoroEngine is the optional neural voice: one sherpa-onnx child per
// sentence writing its WAV to stdout.
type KokoroEngine struct {
	bin, dir string
	lexicons []string
	speaker  int
	speed    float64
	run      Runner
}

// FindKokoro looks for the sherpa-onnx program and a complete pack; it
// fails with ErrUnavailable when either is missing. Nothing is started.
func FindKokoro(o KokoroOptions) (*KokoroEngine, error) {
	bin := o.Binary
	if bin == "" {
		bin = KokoroBinary
	}
	bin, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, KokoroBinary)
	}
	dir := o.Dir
	if dir == "" {
		dir = KokoroDir()
	}
	if name := KokoroPackMissing(dir); name != "" {
		return nil, fmt.Errorf("%w: the Kokoro pack in %s has no %s", ErrUnavailable, dir, name)
	}
	lexicons, _ := filepath.Glob(filepath.Join(dir, "lexicon*.txt"))
	speed := o.Speed
	if speed == 0 {
		speed = 1
	}
	run := o.Run
	if run == nil {
		run = Exec
	}
	return &KokoroEngine{bin: bin, dir: dir, lexicons: lexicons, speaker: o.Speaker, speed: speed, run: run}, nil
}

// KokoroPackMissing names the first file a complete Kokoro pack in dir
// lacks, "" when it holds them all. It reads only the folder.
func KokoroPackMissing(dir string) string {
	for _, f := range []struct {
		name   string
		folder bool
	}{{kokoroModel, false}, {kokoroVoices, false}, {kokoroTokens, false}, {kokoroData, true}} {
		info, err := os.Stat(filepath.Join(dir, f.name))
		if err != nil || info.IsDir() != f.folder || (!f.folder && info.Size() == 0) {
			return f.name
		}
	}
	return ""
}

func (e *KokoroEngine) Name() string { return Kokoro }

func (e *KokoroEngine) Voice() string {
	return "kokoro|" + e.bin + "|" + e.dir + "|" + strconv.Itoa(e.speaker) + "|" + strconv.FormatFloat(e.speed, 'g', -1, 64)
}

// Render speaks line through sherpa-onnx. The text is the last argument,
// so a line never starts with a dash; the WAV is read from stdout, past
// anything the program logs before it.
func (e *KokoroEngine) Render(ctx context.Context, line string) ([]float32, int, error) {
	line = strings.TrimLeft(strings.TrimSpace(line), "-")
	if line == "" {
		return nil, 0, fmt.Errorf("tts: nothing to say")
	}
	argv := []string{
		e.bin,
		"--kokoro-model=" + filepath.Join(e.dir, kokoroModel),
		"--kokoro-voices=" + filepath.Join(e.dir, kokoroVoices),
		"--kokoro-tokens=" + filepath.Join(e.dir, kokoroTokens),
		"--kokoro-data-dir=" + filepath.Join(e.dir, kokoroData),
		"--kokoro-length-scale=" + strconv.FormatFloat(1/e.speed, 'f', 3, 64),
		"--num-threads=" + strconv.Itoa(kokoroThreads),
		"--sid=" + strconv.Itoa(e.speaker),
		"--output-filename=/dev/stdout",
	}
	if len(e.lexicons) > 0 {
		argv = append(argv, "--kokoro-lexicon="+strings.Join(e.lexicons, ","))
	}
	argv = append(argv, line)
	ctx, cancel := context.WithTimeout(ctx, renderLimit)
	defer cancel()
	out, err := e.run(ctx, argv, nil)
	if err != nil {
		return nil, 0, err
	}
	at := bytes.Index(out, []byte("RIFF"))
	if at < 0 {
		return nil, 0, fmt.Errorf("tts: %s wrote no WAV", KokoroBinary)
	}
	pcm, rate, err := clips.DecodeWAV(out[at:])
	if err != nil {
		return nil, 0, fmt.Errorf("tts: %s: %w", KokoroBinary, err)
	}
	return clips.Resample(pcm, rate, SpeechRateHz()), SpeechRateHz(), nil
}
