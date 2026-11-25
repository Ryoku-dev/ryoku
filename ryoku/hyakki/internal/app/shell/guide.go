package shell

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/ocr"
)

// demoNoOCR is the guide demo on a machine without a text reader
const demoNoOCR = "oni-guide-no-ocr"

// guideHost is what the guide settings read: the keeps file the daemon
// writes and whether tesseract is installed, or, when the demo daemon
// answers, the demo's keeps beside its socket and its fixed machine
func (r *runtime) guideHost() env.Guide {
	if r.demoKeeps != "" {
		missing := ""
		if r.demoNoOCR {
			missing = ocrMissing()
		}
		return env.Guide{Keeps: keep.NewStore(r.demoKeeps), OCR: func() string { return missing }}
	}
	return env.Guide{Keeps: keep.NewStore(""), OCR: func() string {
		if !ocr.Available() {
			return ocrMissing()
		}
		return ""
	}}
}

func ocrMissing() string {
	return i18n.Tf("%s is not installed, so Oni cannot read the text in a window.", ocr.Tool)
}

// the guide demo's sessions start it with guides and Keep on
func guideSession(session string) bool {
	return session == demo.FeatureOniGuide || session == demoNoOCR
}
