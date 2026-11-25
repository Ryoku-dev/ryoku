package localmodels

import (
	"net/url"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// File is pinned by checksum for owned models; linked models use size and mtime.
// Role is protocol.LocalFileModel or protocol.LocalFileMmproj.
type File struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	ModTime int64  `json:"modTime,omitempty"`
}

// Entry is a curated, downloaded, imported or linked model. Linked entries
// reference original files through External; they never own those files.
// Args are extra router preset keys (llama-server flags without dashes).
type Entry struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Repo      string                `json:"repo"`
	Revision  string                `json:"revision"`
	Files     []File                `json:"files"`
	External  string                `json:"external,omitempty"`
	License   protocol.LocalLicense `json:"license"`
	Params    int64                 `json:"params,omitempty"`
	Context   int                   `json:"context,omitempty"`
	Class     string                `json:"class,omitempty"`
	Tools     bool                  `json:"tools,omitempty"`
	Vision    bool                  `json:"vision,omitempty"`
	Audio     bool                  `json:"audio,omitempty"`
	Video     bool                  `json:"video,omitempty"`
	Reasoning bool                  `json:"reasoning,omitempty"`
	Args      map[string]string     `json:"args,omitempty"`
	Catalogue bool                  `json:"-"`
}

// DefaultContext is the context a model without a catalogue value gets, so
// --fit does not reserve the largest window the device allows.
const DefaultContext = 32768

// lfm3Revision pins the LFM3 GGUF repo; the URLs and hashes below belong to
// it.
const lfm3Revision = "d508b88f08a5b4286f2d2805a6a8d69a0ff44c72"

// catalogue is the curated set, smallest first.
var catalogue = []Entry{
	{
		ID:       "lfm3-omni-2b",
		Name:     "LFM3 Omni 2B",
		Repo:     "LiquidAI/LFM3-Omni-2B-GGUF",
		Revision: lfm3Revision,
		Files: []File{
			{Name: "LFM3-Omni-2B-Q4_K_M.gguf", Role: protocol.LocalFileModel, Size: 1099630080, SHA256: "9f8d30e9d4fd7a9349d586e077988229b3ca0a0e4ad730897563d1eccc9893e2"},
			{Name: "mmproj-LFM3-Omni-2B-Q4_K_M.gguf", Role: protocol.LocalFileMmproj, Size: 291586720, SHA256: "0a746c0794782a16d4456b230c2a0b5888a2d94a6677211edbed7d7a4177a39a"},
		},
		License: protocol.LocalLicense{
			Name: "LFM Open License v1.0",
			URL:  "https://huggingface.co/LiquidAI/LFM3-Omni-2B-GGUF/blob/" + lfm3Revision + "/LICENSE",
			Note: "Commercial use is licensed only below US$10M annual revenue.",
		},
		Params:  2_000_000_000,
		Context: 32768,
		Class:   protocol.LocalClassLight,
		Tools:   true,
		Vision:  true,
		Audio:   true,
		Video:   true,
		// the card: a non-reasoning model, greedy decoding for text
		Args: map[string]string{"reasoning": "off", "temp": "0"},
	},
}

// Catalogue returns a copy of the curated models.
func Catalogue() []Entry {
	out := make([]Entry, len(catalogue))
	for i, e := range catalogue {
		out[i] = e.clone()
		out[i].Catalogue = true
	}
	return out
}

func (e Entry) clone() Entry {
	e.Files = append([]File(nil), e.Files...)
	if e.Args != nil {
		args := make(map[string]string, len(e.Args))
		for k, v := range e.Args {
			args[k] = v
		}
		e.Args = args
	}
	return e
}

// model is the entry's main GGUF.
func (e Entry) model() (File, bool) {
	for _, f := range e.Files {
		if f.Role == protocol.LocalFileModel {
			return f, true
		}
	}
	return File{}, false
}

// mmproj is the entry's projector, when it has one.
func (e Entry) mmproj() (File, bool) {
	for _, f := range e.Files {
		if f.Role == protocol.LocalFileMmproj {
			return f, true
		}
	}
	return File{}, false
}

// size is the bytes of every file.
func (e Entry) size() int64 {
	var n int64
	for _, f := range e.Files {
		n += f.Size
	}
	return n
}

// url is where a file of the entry downloads from, at the pinned revision.
func (e Entry) url(base, name string) string {
	parts := strings.Split(name, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.TrimRight(base, "/") + "/" + e.Repo + "/resolve/" + e.Revision + "/" + strings.Join(parts, "/")
}

// info is the entry as the protocol shows it, before state is known.
func (e Entry) info() protocol.LocalModel {
	m := protocol.LocalModel{
		ID: e.ID, Name: e.Name, Repo: e.Repo, Revision: e.Revision, License: e.License,
		Params: e.Params, Context: e.Context, Class: e.Class,
		Tools: e.Tools, Vision: e.Vision, Audio: e.Audio, Video: e.Video, Reasoning: e.Reasoning,
		Catalogue: e.Catalogue, State: protocol.LocalAvailable,
	}
	if !e.Catalogue {
		m.Warning = modelWarning(ggufMetadata{Params: m.Params, Context: m.Context, Tools: m.Tools})
	}
	for _, f := range e.Files {
		m.Files = append(m.Files, protocol.LocalFile{Name: f.Name, Role: f.Role, Size: f.Size, SHA256: f.SHA256})
	}
	return m
}
