package protocol

import "time"

// MemberConfigHistoryLimit includes the current snapshot and its prior versions.
const MemberConfigHistoryLimit = 32

type MemberConfigRef struct {
	Team     string `json:"team"`
	Member   string `json:"member"`
	Revision uint64 `json:"revision,omitempty"`
}

type MemberConfigSaveParams struct {
	Team             string `json:"team"`
	Member           string `json:"member"`
	YAML             string `json:"yaml"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

type MemberConfigRestoreParams struct {
	Team             string `json:"team"`
	Member           string `json:"member"`
	Revision         uint64 `json:"revision"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

type MemberConfigDocument struct {
	YAML            string    `json:"yaml"`
	Revision        uint64    `json:"revision"`
	CurrentRevision uint64    `json:"currentRevision"`
	Updated         time.Time `json:"updated"`
}

type MemberConfigRevision struct {
	Revision uint64    `json:"revision"`
	At       time.Time `json:"at"`
	Name     string    `json:"name"`
	Current  bool      `json:"current"`
}

type MemberConfigHistoryResult struct {
	Revisions      []MemberConfigRevision `json:"revisions"`
	RetentionLimit int                    `json:"retentionLimit"`
}
