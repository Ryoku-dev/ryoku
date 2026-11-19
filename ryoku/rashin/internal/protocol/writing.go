package protocol

type WritingCheckParams struct {
	Request  string   `json:"request,omitempty"`
	Text     string   `json:"text"`
	Kinds    []string `json:"kinds,omitempty"`
	Engine   string   `json:"engine,omitempty"`
	Dialect  string   `json:"dialect,omitempty"`
	Off      []string `json:"off,omitempty"`
	Language string   `json:"language,omitempty"`
	Model    string   `json:"model,omitempty"`
	Local    bool     `json:"local,omitempty"`
}

type WritingIssue struct {
	ID           string   `json:"id"`
	Start        int      `json:"start"`
	End          int      `json:"end"`
	Kind         string   `json:"kind"`
	Category     string   `json:"category"`
	Message      string   `json:"message"`
	Replacements []string `json:"replacements,omitempty"`
	Severity     string   `json:"severity"`
}

type WritingStats struct {
	Words       int     `json:"words"`
	Sentences   int     `json:"sentences"`
	ReadingEase float64 `json:"readingEase"`
	Grade       float64 `json:"grade"`
	Minutes     float64 `json:"minutes"`
}

type WritingSignal struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Detail string  `json:"detail,omitempty"`
}

type WritingAITell struct {
	Score   int             `json:"score"`
	Verdict string          `json:"verdict"`
	Signals []WritingSignal `json:"signals"`
	Phrases []WritingIssue  `json:"phrases"`
}

type WritingCheckResult struct {
	Engine string         `json:"engine"`
	Issues []WritingIssue `json:"issues"`
	Stats  WritingStats   `json:"stats"`
	AITell *WritingAITell `json:"aitell,omitempty"`
}

type WritingTransformParams struct {
	Request  string            `json:"request,omitempty"`
	Text     string            `json:"text"`
	Action   string            `json:"action"`
	Options  map[string]string `json:"options,omitempty"`
	Lines    []string          `json:"lines,omitempty"`
	Language string            `json:"language,omitempty"`
	Source   string            `json:"source,omitempty"`
	Target   string            `json:"target,omitempty"`
	Model    string            `json:"model,omitempty"`
	Local    bool              `json:"local,omitempty"`
}

type WritingTransformResult struct {
	Text  string   `json:"text"`
	Lines []string `json:"lines,omitempty"`
	Model string   `json:"model,omitempty"`
}

type WritingVoice struct {
	Samples string `json:"samples"`
	Notes   string `json:"notes"`
	Updated string `json:"updated,omitempty"`
}

type WritingVoiceGetParams struct{}

type WritingVoiceSetParams struct {
	Samples *string `json:"samples,omitempty"`
	Notes   *string `json:"notes,omitempty"`
}

type WritingVoiceLearnParams struct {
	Request string `json:"request,omitempty"`
	Model   string `json:"model,omitempty"`
	Local   bool   `json:"local,omitempty"`
}

type WritingCancelParams struct {
	Request string `json:"request"`
}

type WritingWordsParams struct{}

type WritingWordParams struct {
	Word string `json:"word"`
}

type WritingWords struct {
	Words []string `json:"words"`
}
