package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	DecisionRules           = "rules"
	DecisionLocal           = "local"
	DecisionEmber           = "ember"
	DecisionAuto            = "auto"
	EmberDecisionModel      = "ember-decisions"
	DefaultDecisionAddress  = "http://127.0.0.1:11435/v1"
	DefaultDecisionBudgetMS = 1500
	DecisionBodyMax         = 256 << 10
	DecisionResponseMax     = 512 << 10
)

// DecisionRoutingSettings selects optional accelerators. The rules-only default
// starts nothing; neither an answer nor a confidence value is an action grant.
type DecisionRoutingSettings struct {
	Route        string            `json:"route,omitempty"`
	LocalAddress string            `json:"localAddress,omitempty"`
	LocalModel   string            `json:"localModel,omitempty"`
	BudgetMS     int               `json:"budgetMs,omitempty"`
	AllowHosted  bool              `json:"allowHosted,omitempty"`
	Features     map[string]string `json:"features,omitempty"`
}

// What asking for the local decision server found. Detected is the only
// state in which the local route is asked; the rest say why it is not.
const (
	DecisionLocalDetected = "detected"
	DecisionLocalAbsent   = "absent"
	DecisionLocalSmall    = "small"
	DecisionLocalRefused  = "refused"
)

// DecisionLocalStatus is one on-demand look for the local decision server
// the user runs: never polled, and never anything but a loopback address.
// RAMMiB and Cores are what the hardware gate read; Reason is a plain
// sentence for every state but detected.
type DecisionLocalStatus struct {
	State   string `json:"state"`
	Address string `json:"address,omitempty"`
	RAMMiB  int    `json:"ramMiB,omitempty"`
	Cores   int    `json:"cores,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type DecisionQuestion struct {
	Type         string          `json:"type"`
	Instructions json.RawMessage `json:"instructions,omitempty"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

// DecisionRequest is shared by the local and hosted native decision endpoints.
// State and instructions are data, never system instructions or executable work.
type DecisionRequest struct {
	Model     string                      `json:"model"`
	State     json.RawMessage             `json:"state"`
	Questions map[string]DecisionQuestion `json:"questions"`
}

type FastDecisionParams struct {
	DecisionRequest
	Route          string `json:"route,omitempty"`
	Feature        string `json:"feature,omitempty"`
	BudgetMS       int    `json:"budgetMs,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	// AccountID binds paid selection to the identity the app actually reviewed.
	AccountID string `json:"accountID,omitempty"`
}

type DecisionAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
	Noul          *float64           `json:"noul,omitempty"`
}

type DecisionUsage struct {
	Input  int64 `json:"input_tokens"`
	Output int64 `json:"output_tokens"`
}

type DecisionResponse struct {
	Model   string                    `json:"model"`
	Answers map[string]DecisionAnswer `json:"answers"`
	Usage   DecisionUsage             `json:"usage"`
}

// FastDecisionResult is one decision call's outcome. CreditsExhausted says
// the credit route refused for want of Ember credits, so nothing hosted was
// dispatched; it is set whether or not another route then answered, and a
// consumer reads it instead of the notice's wording.
type FastDecisionResult struct {
	Route            string            `json:"route"`
	Resolved         bool              `json:"resolved"`
	Decision         *DecisionResponse `json:"decision,omitempty"`
	Notice           string            `json:"notice"`
	ElapsedMS        int64             `json:"elapsedMs"`
	EmberUnits       int64             `json:"emberUnits,omitempty"`
	CreditsExhausted bool              `json:"creditsExhausted,omitempty"`
}

// DecisionSpec retains parsed criteria so response validation cannot reinterpret
// the question, invent another option or reparse every criterion after a fetch.
type DecisionSpec struct {
	questions map[string]decisionQuestionSpec
	request   DecisionRequest
}

type decisionQuestionSpec struct {
	kind    string
	choices map[string]string
	levels  []string
}

func ValidDecisionRoute(route string) bool {
	return route == DecisionRules || route == DecisionLocal || route == DecisionEmber || route == DecisionAuto
}

func ValidDecisionFeature(feature string) bool {
	if len(feature) < 1 || len(feature) > 64 {
		return false
	}
	for _, r := range feature {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// ParseDecisionAddress is stricter than addressed chat providers: a route
// labelled local never reaches a LAN host, DNS-selected address or proxy.
func ParseDecisionAddress(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		if port := u.Port(); port != "" {
			u.Host = "127.0.0.1:" + port
		} else {
			u.Host = "127.0.0.1"
		}
	} else if ip, err := netip.ParseAddr(host); err != nil || !ip.Unmap().IsLoopback() {
		return "", false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", false
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/v1"
	}
	u.RawPath = ""
	return u.String(), true
}

func decisionText(text string, limit int, empty bool) bool {
	if !utf8.ValidString(text) || len(text) > limit || (!empty && strings.TrimSpace(text) == "") {
		return false
	}
	for _, r := range text {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func decisionData(raw json.RawMessage, optional bool) bool {
	raw = bytes.TrimSpace(raw)
	if optional && (len(raw) == 0 || bytes.Equal(raw, []byte("null"))) {
		return true
	}
	return len(raw) > 0 && (raw[0] == '"' || raw[0] == '{' || raw[0] == '[') && utf8.Valid(raw) && json.Valid(raw)
}

func CompileDecisionRequest(p DecisionRequest) (DecisionSpec, error) {
	var empty DecisionSpec
	if !decisionText(p.Model, 256, false) || strings.ContainsAny(p.Model, " \t\r\n") {
		return empty, fmt.Errorf("Choose a decision model.")
	}
	if !decisionData(p.State, false) || len(p.State) > 64<<10 {
		return empty, fmt.Errorf("Decision state must be text, an object or an array of at most 64 KiB.")
	}
	if len(p.Questions) < 1 || len(p.Questions) > 256 {
		return empty, fmt.Errorf("Ask between 1 and 256 typed questions.")
	}
	spec := DecisionSpec{questions: make(map[string]decisionQuestionSpec, len(p.Questions))}
	spec.request = p
	spec.request.Questions = make(map[string]DecisionQuestion, len(p.Questions))
	size := len(p.State) + len(p.Model)
	for id, q := range p.Questions {
		if !decisionText(id, 128, false) || !decisionData(q.Instructions, true) || len(q.Instructions) > 8<<10 || len(q.Criteria) > 64<<10 {
			return empty, fmt.Errorf("A decision question is malformed or too long.")
		}
		size += len(id) + len(q.Type) + len(q.Instructions) + len(q.Criteria)
		if size > DecisionBodyMax {
			return empty, fmt.Errorf("The decision request is too large.")
		}
		item := decisionQuestionSpec{kind: q.Type}
		switch q.Type {
		case "choice", "noul":
			raw := bytes.TrimSpace(q.Criteria)
			if q.Type == "noul" && (len(raw) == 0 || bytes.Equal(raw, []byte("null"))) {
				break
			}
			if len(raw) > 0 && raw[0] == '[' && q.Type == "choice" {
				var labels []string
				if json.Unmarshal(raw, &labels) != nil {
					return empty, fmt.Errorf("Choice criteria must be labels or descriptions.")
				}
				item.choices = make(map[string]string, len(labels))
				for _, label := range labels {
					if _, exists := item.choices[label]; exists {
						return empty, fmt.Errorf("Choice labels must be unique.")
					}
					item.choices[label] = ""
				}
			} else {
				if json.Unmarshal(raw, &item.choices) != nil {
					return empty, fmt.Errorf("Choice criteria must be labels or descriptions.")
				}
			}
			if q.Type == "noul" {
				if len(item.choices) != 2 {
					return empty, fmt.Errorf("A truth question accepts only true and false descriptions.")
				}
				if _, ok := item.choices["true"]; !ok {
					return empty, fmt.Errorf("A truth question needs true and false criteria.")
				}
				if _, ok := item.choices["false"]; !ok {
					return empty, fmt.Errorf("A truth question needs true and false criteria.")
				}
			} else if len(item.choices) < 2 || len(item.choices) > 255 {
				return empty, fmt.Errorf("A choice question needs 2 to 255 unique options.")
			}
			for label, description := range item.choices {
				if !decisionText(label, 256, false) || !decisionText(description, 4096, true) {
					return empty, fmt.Errorf("A criterion is malformed or too long.")
				}
			}
		case "score":
			if json.Unmarshal(q.Criteria, &item.levels) != nil || len(item.levels) < 2 || len(item.levels) > 10 {
				return empty, fmt.Errorf("A score question needs 2 to 10 level descriptions.")
			}
			for _, level := range item.levels {
				if !decisionText(level, 4096, false) {
					return empty, fmt.Errorf("A score level is malformed or too long.")
				}
			}
		default:
			return empty, fmt.Errorf("A decision question must be choice, score or noul.")
		}
		spec.questions[id] = item
		if q.Type == "noul" && item.choices == nil {
			q.Criteria = nil
		} else {
			var criteria any = item.choices
			if q.Type == "score" {
				criteria = item.levels
			}
			canonical, err := json.Marshal(criteria)
			if err != nil {
				return empty, err
			}
			q.Criteria = canonical
		}
		spec.request.Questions[id] = q
	}
	return spec, nil
}

// Request uses the criteria that were checked, not ambiguous original JSON.
// The returned request is read-only; changing its model does not copy the state.
func (s DecisionSpec) Request(model string) DecisionRequest {
	request := s.request
	request.Model = model
	return request
}

func probability(p float64) bool { return !math.IsNaN(p) && !math.IsInf(p, 0) && p >= 0 && p <= 1 }

// Validate checks semantic correspondence, not model quality or calibration on
// this workload. Consumers still own thresholds, consent and every action.
func (s DecisionSpec) Validate(out DecisionResponse) error {
	if len(s.questions) == 0 || !decisionText(out.Model, 256, false) || len(out.Answers) != len(s.questions) || out.Usage.Input <= 0 || out.Usage.Input > 1<<30 || out.Usage.Output != 0 {
		return fmt.Errorf("The decision service returned an invalid model, answer set or usage.")
	}
	for id, q := range s.questions {
		a, ok := out.Answers[id]
		if !ok || a.Type != q.kind {
			return fmt.Errorf("The decision service answered a different question.")
		}
		if q.kind == "noul" {
			if a.Noul == nil || !probability(*a.Noul) {
				return fmt.Errorf("The decision service returned an invalid truth probability.")
			}
			continue
		}
		n := len(q.choices)
		if q.kind == "score" {
			n = len(q.levels)
		}
		if len(a.Probabilities) != n || a.Confidence == nil || !probability(*a.Confidence) {
			return fmt.Errorf("The decision service returned an incomplete probability distribution.")
		}
		sum, top, expected := 0.0, 0.0, 0.0
		for label, p := range a.Probabilities {
			if !probability(p) {
				return fmt.Errorf("The decision service returned an invalid probability.")
			}
			if q.kind == "choice" {
				if _, ok := q.choices[label]; !ok {
					return fmt.Errorf("The decision service invented an option.")
				}
			} else {
				level, err := strconv.Atoi(label)
				if err != nil || strconv.Itoa(level) != label || level < 0 || level >= n {
					return fmt.Errorf("The decision service invented a score level.")
				}
				expected += float64(level) * p
			}
			sum += p
			top = math.Max(top, p)
		}
		if math.Abs(sum-1) > float64(n)*0.00006+0.000001 || math.Abs(*a.Confidence-(float64(n)*top-1)/float64(n-1)) > 0.001 {
			return fmt.Errorf("The decision service returned inconsistent confidence.")
		}
		if q.kind == "choice" {
			p, exists := a.Probabilities[a.Choice]
			if !exists || p+0.0001 < top {
				return fmt.Errorf("The decision service returned an inconsistent choice.")
			}
		} else {
			if a.Score == nil || math.IsNaN(*a.Score) || math.IsInf(*a.Score, 0) || *a.Score < 0 || *a.Score > float64(n-1) || math.Abs(*a.Score-expected) > 0.005 || len(a.Legend) != n {
				return fmt.Errorf("The decision service returned an inconsistent score.")
			}
			for level, description := range q.levels {
				if a.Legend[strconv.Itoa(level)] != description {
					return fmt.Errorf("The decision service changed the score legend.")
				}
			}
		}
	}
	return nil
}

// DecisionMargin is the gap between the two strongest options, never a grant.
func DecisionMargin(answer DecisionAnswer) float64 {
	if answer.Type == "noul" && answer.Noul != nil {
		return math.Abs(2**answer.Noul - 1)
	}
	if len(answer.Probabilities) < 2 {
		return 0
	}
	first, second := 0.0, 0.0
	for _, p := range answer.Probabilities {
		if p >= first {
			second, first = first, p
		} else if p > second {
			second = p
		}
	}
	return first - second
}
