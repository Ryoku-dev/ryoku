// Package honcho keeps Rashin's memory on a Honcho server through its v3 HTTP
// API. The client is written by hand from the API's field names: the calls
// Rashin makes, a Bearer key, a timeout and the server's status codes.
package honcho

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/memory/backend"
)

// callTimeout bounds one request; a server that does not answer in it counts
// as unreachable.
const callTimeout = 20 * time.Second

// pageSize is the largest page the server gives.
const pageSize = 100

// contentMax is the longest message the server takes; a longer turn is sent
// in parts.
const contentMax = 25000

// errNotFound is a 404 from the server.
var errNotFound = errors.New("not found")

// client sends requests to one Honcho server in one workspace.
type client struct {
	base      string
	key       string
	workspace string
	http      *http.Client
}

func newClient(base, key, workspace string) *client {
	return &client{base: strings.TrimRight(base, "/"), key: key, workspace: workspace, http: &http.Client{Timeout: callTimeout}}
}

// path builds a workspace path from escaped parts.
func (c *client) path(parts ...string) string {
	escaped := []string{"/v3/workspaces", url.PathEscape(c.workspace)}
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return strings.Join(escaped, "/")
}

// do sends one request with in as its JSON body and decodes the answer into
// out. Any 2xx status is success; 404 is errNotFound; 422 says what the
// server refused.
func (c *client) do(ctx context.Context, method, path string, query url.Values, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	target := c.base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return unreachable(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return unreachable(err)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return errors.New(i18n.T("Honcho refused the key."))
	case resp.StatusCode == http.StatusUnprocessableEntity:
		return refused(raw)
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return errors.New(i18n.Tf("Honcho answered %d.", resp.StatusCode))
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Honcho sent an answer Rashin cannot read"), err)
	}
	return nil
}

// unreachable words a transport error without the request's address.
func unreachable(err error) error {
	return fmt.Errorf("%s: %w", i18n.T("Honcho cannot be reached"), backend.Bare(err))
}

// refused reads a 422's first complaint.
func refused(raw []byte) error {
	var answer struct {
		Detail json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(raw, &answer) == nil {
		var details []struct {
			Loc []any  `json:"loc"`
			Msg string `json:"msg"`
		}
		if json.Unmarshal(answer.Detail, &details) == nil && len(details) > 0 {
			return errors.New(i18n.Tf("Honcho refused the request: %s", details[0].Msg))
		}
		var text string
		if json.Unmarshal(answer.Detail, &text) == nil && text != "" {
			return errors.New(i18n.Tf("Honcho refused the request: %s", text))
		}
	}
	return errors.New(i18n.T("Honcho refused the request."))
}

// conclusion is a fact Honcho holds: what observer concluded about observed.
type conclusion struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	ObserverID string    `json:"observer_id"`
	ObservedID string    `json:"observed_id"`
	SessionID  *string   `json:"session_id,omitempty"`
	Level      string    `json:"level,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type conclusionCreate struct {
	Content    string `json:"content"`
	ObserverID string `json:"observer_id"`
	ObservedID string `json:"observed_id"`
	SessionID  string `json:"session_id,omitempty"`
}

type peerConfig struct {
	ObserveMe     *bool `json:"observe_me,omitempty"`
	ObserveOthers *bool `json:"observe_others,omitempty"`
}

type sessionCreate struct {
	ID       string                `json:"id"`
	Metadata map[string]any        `json:"metadata,omitempty"`
	Peers    map[string]peerConfig `json:"peers,omitempty"`
}

type messageCreate struct {
	Content   string         `json:"content"`
	PeerID    string         `json:"peer_id"`
	CreatedAt time.Time      `json:"created_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Pages int `json:"pages"`
}

type filters struct {
	Filters map[string]any `json:"filters,omitempty"`
}

type queueStatus struct {
	Pending    int `json:"pending_work_units"`
	InProgress int `json:"in_progress_work_units"`
}

// ensureWorkspace gets or makes the client's workspace.
func (c *client) ensureWorkspace(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/v3/workspaces", nil, map[string]string{"id": c.workspace}, nil)
}

// ensurePeer gets or makes a peer.
func (c *client) ensurePeer(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, c.path("peers"), nil, map[string]string{"id": id}, nil)
}

func (c *client) createConclusions(ctx context.Context, in []conclusionCreate) ([]conclusion, error) {
	var out []conclusion
	err := c.do(ctx, http.MethodPost, c.path("conclusions"), nil, map[string]any{"conclusions": in}, &out)
	return out, err
}

func (c *client) getConclusion(ctx context.Context, id string) (conclusion, error) {
	var out conclusion
	err := c.do(ctx, http.MethodGet, c.path("conclusions", id), nil, nil, &out)
	return out, err
}

// deleteConclusion deletes one conclusion; one already gone counts as
// deleted.
func (c *client) deleteConclusion(ctx context.Context, id string) error {
	if err := c.do(ctx, http.MethodDelete, c.path("conclusions", id), nil, nil, nil); err != nil && !errors.Is(err, errNotFound) {
		return err
	}
	return nil
}

// listConclusions returns one page, counted from 1.
func (c *client) listConclusions(ctx context.Context, f map[string]any, number, size int) (page[conclusion], error) {
	var out page[conclusion]
	query := url.Values{"page": {fmt.Sprint(number)}, "size": {fmt.Sprint(size)}}
	err := c.do(ctx, http.MethodPost, c.path("conclusions", "list"), query, filters{Filters: f}, &out)
	return out, err
}

func (c *client) queryConclusions(ctx context.Context, text string, topK int, f map[string]any) ([]conclusion, error) {
	var out []conclusion
	err := c.do(ctx, http.MethodPost, c.path("conclusions", "query"), nil, map[string]any{"query": text, "top_k": topK, "filters": f}, &out)
	return out, err
}

func (c *client) ensureSession(ctx context.Context, s sessionCreate) error {
	return c.do(ctx, http.MethodPost, c.path("sessions"), nil, s, nil)
}

func (c *client) addMessages(ctx context.Context, session string, messages []messageCreate) error {
	return c.do(ctx, http.MethodPost, c.path("sessions", session, "messages"), nil, map[string]any{"messages": messages}, nil)
}

// messageKeys returns the keyField values of the messages a session already
// holds; a session not made yet holds none.
func (c *client) messageKeys(ctx context.Context, session string) (map[string]bool, error) {
	keys := map[string]bool{}
	for number := 1; ; number++ {
		var out page[struct {
			Metadata map[string]any `json:"metadata"`
		}]
		query := url.Values{"page": {fmt.Sprint(number)}, "size": {fmt.Sprint(pageSize)}}
		err := c.do(ctx, http.MethodPost, c.path("sessions", session, "messages", "list"), query, filters{}, &out)
		if errors.Is(err, errNotFound) {
			return keys, nil
		}
		if err != nil {
			return nil, err
		}
		for _, m := range out.Items {
			if key, ok := m.Metadata[keyField].(string); ok {
				keys[key] = true
			}
		}
		if number >= out.Pages || len(out.Items) == 0 {
			return keys, nil
		}
	}
}

// deleteSession asks the server to delete a session and what it derived
// from it; the server finishes on its own clock.
func (c *client) deleteSession(ctx context.Context, id string) error {
	if err := c.do(ctx, http.MethodDelete, c.path("sessions", id), nil, nil, nil); err != nil && !errors.Is(err, errNotFound) {
		return err
	}
	return nil
}

// peerSessions returns one page of a peer's sessions' ids, counted from 1.
func (c *client) peerSessions(ctx context.Context, peer string, number int) (page[struct {
	ID string `json:"id"`
}], error) {
	var out page[struct {
		ID string `json:"id"`
	}]
	query := url.Values{"page": {fmt.Sprint(number)}, "size": {fmt.Sprint(pageSize)}}
	err := c.do(ctx, http.MethodPost, c.path("peers", peer, "sessions"), query, filters{}, &out)
	return out, err
}

// peers returns one page of the workspace's peers' ids, counted from 1.
func (c *client) peers(ctx context.Context, number int) (page[struct {
	ID string `json:"id"`
}], error) {
	var out page[struct {
		ID string `json:"id"`
	}]
	query := url.Values{"page": {fmt.Sprint(number)}, "size": {fmt.Sprint(pageSize)}}
	err := c.do(ctx, http.MethodPost, c.path("peers", "list"), query, filters{}, &out)
	return out, err
}

func (c *client) card(ctx context.Context, peer, target string) ([]string, error) {
	var out struct {
		PeerCard []string `json:"peer_card"`
	}
	err := c.do(ctx, http.MethodGet, c.path("peers", peer, "card"), url.Values{"target": {target}}, nil, &out)
	return out.PeerCard, err
}

func (c *client) queue(ctx context.Context) (queueStatus, error) {
	var out queueStatus
	err := c.do(ctx, http.MethodGet, c.path("queue", "status"), nil, nil, &out)
	return out, err
}

func (c *client) health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/health", nil, nil, nil)
}
