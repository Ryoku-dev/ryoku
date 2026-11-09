package term

import "github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

// Claim describes a state word and its flake.
type Claim struct {
	Tone     palette.Tone
	Word     string
	Progress float64
}

// RenderClaim renders state as mineral, mark, and word.
func RenderClaim(th Theme, claim Claim) string {
	word := claim.Word
	if word == "" {
		word = toneName(claim.Tone)
	}
	colour := th.Roles.Stone(claim.Tone)
	if claim.Tone == palette.Idle {
		colour = th.Roles.Fg2
	}
	return RenderFlake(th, claim.Tone, claim.Progress) + " " + ink(colour).Render(word)
}
