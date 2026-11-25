package protocol

const EvCompare = "compare"

type CompareStartParams struct {
	ID     string   `json:"id"`
	Seq    int64    `json:"seq"`
	Models []string `json:"models"`
}

type CompareLane struct {
	Session string `json:"session"`
	Model   string `json:"model"`
}

type CompareStartResult struct {
	ID     string        `json:"id"`
	Source string        `json:"source"`
	Seq    int64         `json:"seq"`
	Lanes  []CompareLane `json:"lanes"`
}

type CompareKeepParams struct {
	ID      string `json:"id"`
	Session string `json:"session"`
}
