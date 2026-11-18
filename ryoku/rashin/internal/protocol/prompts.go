package protocol

type Prompt struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	BuiltIn bool   `json:"builtIn"`
	Updated string `json:"updated,omitempty"`
}

type PromptsListParams struct{}

type PromptsListResult struct {
	Prompts []Prompt `json:"prompts"`
}

type PromptSaveParams struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type PromptDeleteParams struct {
	ID string `json:"id"`
}

type PromptExpandParams struct {
	ID   string            `json:"id"`
	Vars map[string]string `json:"vars,omitempty"`
}

type PromptExpandResult struct {
	Text string `json:"text"`
}
