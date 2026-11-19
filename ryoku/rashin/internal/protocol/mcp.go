package protocol

import "time"

type MCPListParams struct{}
type MCPListResult struct {
	Servers []MCPServerInfo `json:"servers"`
}
type MCPOAuth struct {
	ClientID string `json:"clientId"`
	AuthURL  string `json:"authUrl"`
	TokenURL string `json:"tokenUrl"`
	Scopes   string `json:"scopes,omitempty"`
}

// MCPServer is write-only; header values are sealed before the configuration is saved.
type MCPServer struct {
	ID      string            `json:"id,omitempty"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	OAuth   *MCPOAuth         `json:"oauth,omitempty"`
	Enabled bool              `json:"enabled"`
}

// MCPServerInfo deliberately has no field capable of carrying a credential value.
type MCPServerInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Command     string    `json:"command,omitempty"`
	Args        []string  `json:"args,omitempty"`
	URL         string    `json:"url,omitempty"`
	HeaderNames []string  `json:"headerNames,omitempty"`
	OAuth       *MCPOAuth `json:"oauth,omitempty"`
	Enabled     bool      `json:"enabled"`
	State       string    `json:"state"`
	Tools       int       `json:"tools"`
	Checked     time.Time `json:"checked,omitempty"`
	Detail      string    `json:"detail,omitempty"`
}
type MCPDeleteParams struct {
	ID string `json:"id"`
}
type MCPCheckParams struct {
	ID string `json:"id"`
}
type MCPSignInParams struct {
	ID string `json:"id"`
}
type MCPSignInResult struct {
	URL string `json:"url"`
}
