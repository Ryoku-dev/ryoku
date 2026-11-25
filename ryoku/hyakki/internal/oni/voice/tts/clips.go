package tts

import (
	"context"
	"fmt"

	"ryoku-rashin/internal/oni/voice/clips"
)

// ClipsEngine speaks from a recorded clip pack. A sentence the pack cannot
// say whole fails with clips.ErrMissingClip, so the Speaker moves on to the
// next engine instead of speaking part of it.
type ClipsEngine struct {
	pack *clips.Pack
	id   string
}

// NewClips speaks from pack; id names the pack in the line cache key (its
// folder, for example).
func NewClips(pack *clips.Pack, id string) *ClipsEngine { return &ClipsEngine{pack: pack, id: id} }

func (e *ClipsEngine) Name() string  { return Clips }
func (e *ClipsEngine) Voice() string { return "clips|" + e.id }

func (e *ClipsEngine) Render(ctx context.Context, line string) ([]float32, int, error) {
	refs, err := e.pack.Speak(line)
	if err != nil {
		return nil, 0, err
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	pcm, err := e.pack.PCM(refs, SpeechRateHz())
	if err != nil {
		return nil, 0, fmt.Errorf("tts: clips: %w", err)
	}
	return pcm, SpeechRateHz(), nil
}
