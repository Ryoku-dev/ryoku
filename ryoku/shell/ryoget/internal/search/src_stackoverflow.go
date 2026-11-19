package search

import (
	"context"
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"ryoku-i18n"
)

var stackoverflowSource = &source{
	SourceInfo: SourceInfo{ID: "stackoverflow", Name: "Stack Overflow", Scope: "code", Glyph: "help"},
	limit:      3,
	gate:       gateMedium,
	modes:      modeFull,
	search:     searchStackOverflow,
	detail:     stackoverflowDetail,
}

// a public, immutable API filter that adds title, body and link to answers
const soAnswerFilter = "!)RL-JogHwoZuazxH)kDUxLEa"

type soWrapper struct {
	Items          json.RawMessage `json:"items"`
	HasMore        bool            `json:"has_more"`
	QuotaRemaining *int            `json:"quota_remaining"`
	Backoff        int             `json:"backoff"`
	ErrorID        int             `json:"error_id"`
	ErrorName      string          `json:"error_name"`
	ErrorMessage   string          `json:"error_message"`
}

var soWaitSeconds = regexp.MustCompile(`available in (\d+) seconds`)

func (e *Service) soGet(ctx context.Context, path string, params url.Values, v any) (bool, error) {
	params.Set("site", "stackoverflow")
	u := "https://api.stackexchange.com/2.3/" + path + "?" + params.Encode()
	resp, err := e.fetch(ctx, request{engine: "stackexchange", url: u, accept: "application/json"})
	if err != nil {
		return false, err
	}
	var w soWrapper
	if jerr := json.Unmarshal(resp.body, &w); jerr != nil {
		if serr := statusErr(resp.status); serr != nil {
			return false, serr
		}
		return false, jerr
	}
	now := e.now()
	// Stack Exchange throttles with a 400 and says how long to wait
	if w.ErrorName == "throttle_violation" {
		var wait time.Duration
		if m := soWaitSeconds.FindStringSubmatch(w.ErrorMessage); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				wait = time.Duration(n) * time.Second
			}
		}
		return false, &pausedError{engine: "stackexchange", until: e.gov.strike("stackexchange", wait)}
	}
	if err := statusErr(resp.status); err != nil {
		return false, err
	}
	if w.Backoff > 0 {
		e.gov.hold("stackexchange", time.Duration(w.Backoff)*time.Second)
	}
	if w.QuotaRemaining != nil && *w.QuotaRemaining == 0 {
		next := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
		e.gov.hold("stackexchange", next.Sub(now))
	}
	if err := json.Unmarshal(w.Items, v); err != nil {
		return false, err
	}
	return w.HasMore, nil
}

type soQuestion struct {
	QuestionID       int64    `json:"question_id"`
	Title            string   `json:"title"`
	Excerpt          string   `json:"excerpt"`
	ItemType         string   `json:"item_type"`
	Score            int64    `json:"score"`
	QuestionScore    *int64   `json:"question_score"`
	AnswerCount      int64    `json:"answer_count"`
	HasAccepted      bool     `json:"has_accepted_answer"`
	AcceptedAnswerID int64    `json:"accepted_answer_id"`
	IsAnswered       bool     `json:"is_answered"`
	Tags             []string `json:"tags"`
	CreationDate     int64    `json:"creation_date"`
}

func searchStackOverflow(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	var qs []soQuestion
	more, err := e.soGet(ctx, "search/excerpts", url.Values{
		"q": {q.Text}, "order": {"desc"}, "sort": {"relevance"}, "pagesize": {"10"},
	}, &qs)
	if err != nil {
		return nil, false, err
	}
	// full-text search wants every word; a loose title match still helps
	// for long, wordy queries
	if len(qs) == 0 && len(q.words) > 1 {
		if more, err = e.soGet(ctx, "similar", url.Values{
			"title": {q.Text}, "order": {"desc"}, "sort": {"relevance"}, "pagesize": {"10"},
		}, &qs); err != nil {
			return nil, false, err
		}
	}
	return soItems(qs), more, nil
}

func soItems(qs []soQuestion) []Item {
	seen := map[int64]bool{}
	items := make([]Item, 0, len(qs))
	for _, s := range qs {
		if s.QuestionID == 0 || seen[s.QuestionID] {
			continue
		}
		seen[s.QuestionID] = true
		score := s.Score
		if s.QuestionScore != nil {
			score = *s.QuestionScore
		}
		accepted := s.HasAccepted || s.AcceptedAnswerID != 0
		id := strconv.FormatInt(s.QuestionID, 10)
		items = append(items, Item{
			Key: id, Kind: KindQuestion, Title: plain(html.UnescapeString(s.Title)),
			URL:     "https://stackoverflow.com/questions/" + id,
			Snippet: clip(clean(s.Excerpt), 240), Site: "stackoverflow.com", Date: unixDate(s.CreationDate),
			Meta: map[string]any{"score": score, "answers": s.AnswerCount, "accepted": accepted, "tags": nonNil(s.Tags)},
			pop:  popularity(float64(score), 3),
		})
	}
	return items
}

var digits = regexp.MustCompile(`^\d{1,12}$`)

func stackoverflowDetail(ctx context.Context, e *Service, key, _ string) (Detail, error) {
	if !digits.MatchString(key) {
		return Detail{}, errBadKey
	}
	var answers []struct {
		Title      string `json:"title"`
		Body       string `json:"body"`
		Link       string `json:"link"`
		Score      int64  `json:"score"`
		IsAccepted bool   `json:"is_accepted"`
	}
	if _, err := e.soGet(ctx, "questions/"+key+"/answers", url.Values{
		"order": {"desc"}, "sort": {"votes"}, "pagesize": {"1"}, "filter": {soAnswerFilter},
	}, &answers); err != nil {
		return Detail{}, err
	}
	d := Detail{URL: "https://stackoverflow.com/questions/" + key, Site: "stackoverflow.com"}
	if len(answers) == 0 {
		d.Text = i18n.T("No answers yet.")
		return d, nil
	}
	a := answers[0]
	d.Title = plain(html.UnescapeString(a.Title))
	if a.Link != "" {
		d.URL = a.Link
	}
	d.Text = htmlText(a.Body, 1200)
	d.fact(i18n.T("Top answer score"), strconv.FormatInt(a.Score, 10))
	if a.IsAccepted {
		d.fact(i18n.T("Accepted"), i18n.T("Yes"))
	}
	return d, nil
}
