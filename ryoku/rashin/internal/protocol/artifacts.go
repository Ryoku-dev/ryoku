package protocol

// ArtifactOpenParams previews a code block or a file reported by a session.
type ArtifactOpenParams struct {
	Session string `json:"session"`
	Path    string `json:"path,omitempty"`
	Code    string `json:"code,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

type ArtifactOpenResult struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type ArtifactError struct {
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Time    string `json:"time"`
}

type ArtifactErrorsParams struct {
	ID string `json:"id"`
}

type ArtifactErrorsResult struct {
	Errors []ArtifactError `json:"errors"`
}

type ArtifactRepairParams struct {
	ID string `json:"id"`
}

type ArtifactCloseParams struct {
	ID string `json:"id"`
}
