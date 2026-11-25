package env

import "ryoku-rashin/internal/dictate"

// Voice is what Oni's voice settings read and fetch on this computer. The
// daemon runs the voice loop; Settings only needs to know what is installed,
// to fetch a speech model the user agreed to, and the dictation store the
// daemon reads. A render or a page without it leaves the hooks nil.
type Voice struct {
	// Survey reads, once per call, which voices (character's clips among
	// them), speech engines and speech models are on this computer and
	// whether the chord reader can open the keyboards; done runs on the
	// frame goroutine. Nothing polls it.
	Survey func(character string, done func(VoiceSurvey))
	// Download fetches the catalogue speech model id after the user agreed
	// to its size, licence and hash, pinned to that hash; progress and done
	// run on the frame goroutine, and cancel stops it keeping what it has.
	Download func(id string, progress func(done int64), done func(error)) (cancel func())
	// Dictation keeps the dictionary, snippets and per-app styles.
	Dictation *dictate.Store
}

// VoiceSurvey is one reading of what the voice settings can offer. Each map
// holds the reason a choice cannot be used, "" when it can.
type VoiceSurvey struct {
	// Voices by protocol.OniTTSEngines
	Voices map[string]string
	// Engines by protocol.OniSTTEngines
	Engines map[string]string
	// Models are the speech catalogue ids found on this computer
	Models map[string]bool
	// Keys is why the chord reader cannot read the keyboard, "" when it can
	Keys string
}
