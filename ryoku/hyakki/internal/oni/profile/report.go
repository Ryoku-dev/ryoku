package profile

import (
	"fmt"
	"strings"
)

// Fallback is one slot the loader could not fill from the character and what
// it used in its place.
type Fallback struct {
	Slot   string `json:"slot"`
	Reason string `json:"reason"`
	Used   string `json:"used"`
}

// Report is what loading found. Any error rejects the character; fallbacks
// keep it loadable and say which slots were replaced.
type Report struct {
	Fallbacks []Fallback `json:"fallbacks,omitempty"`
	Errors    []string   `json:"errors,omitempty"`
}

// Rejection reasons. Each error reads "<reason>: <detail>".
const (
	ReasonMissingProfile = "missing-profile"
	ReasonBadJSON        = "bad-json"
	ReasonBadVersion     = "bad-version"
	ReasonBadID          = "bad-id"
	ReasonIDMismatch     = "id-mismatch"
	ReasonShadowsShipped = "shadows-shipped"
	ReasonUnknownKey     = "unknown-key"
	ReasonDuplicateKey   = "duplicate-key"
	ReasonAuthorityKey   = "authority-key"
	ReasonUnknownSlot    = "unknown-slot"
	ReasonBadValue       = "bad-value"
	ReasonAbsolutePath   = "absolute-path"
	ReasonEscapingPath   = "escaping-path"
	ReasonBadPath        = "bad-path"
	ReasonSymlink        = "symlink"
	ReasonUnreadable     = "unreadable"
	ReasonTooLarge       = "too-large"
	ReasonTooMany        = "too-many"
	ReasonUnsafeText     = "unsafe-text"
	ReasonBadSVG         = "bad-svg"
	ReasonActiveSVG      = "active-svg"
	ReasonClipUnlicensed = "clip-pack-unlicensed"
	ReasonClipNoSource   = "clip-pack-no-provenance"
)

// Fallback reasons.
const (
	FallbackMissing      = "missing"
	FallbackAssetMissing = "asset-missing"
	FallbackEmpty        = "empty"
)

// What a fallen-back slot uses instead.
const (
	UsedStaticPose  = "static-pose"
	UsedBasePose    = "base-pose"
	UsedNeutralAcks = "neutral-acks"
	UsedSilhouette  = "default-silhouette"
	UsedNoPersona   = "no-style-block"
	UsedBaseArtwork = "base-artwork"
)

// Has reports whether the report rejects the character for reason.
func (r Report) Has(reason string) bool {
	for _, e := range r.Errors {
		if strings.HasPrefix(e, reason+": ") {
			return true
		}
	}
	return false
}

func (r *Report) reject(reason, format string, args ...any) {
	r.Errors = append(r.Errors, reason+": "+fmt.Sprintf(format, args...))
}

func (r *Report) fallback(slot, reason, used string) {
	r.Fallbacks = append(r.Fallbacks, Fallback{Slot: slot, Reason: reason, Used: used})
}
