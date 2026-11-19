package protocol

type SessionFlagsParams struct {
	ID       string `json:"id"`
	Starred  *bool  `json:"starred,omitempty"`
	Archived *bool  `json:"archived,omitempty"`
}
