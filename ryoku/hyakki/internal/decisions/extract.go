package decisions

// Derived from oh-my-pi (MIT), packages/coding-agent/src/sharpshooter/extract.ts@c7fff67

import (
	"context"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/redact"
)

const (
	// MinPrompt is the fewest characters a prompt needs to be read for
	// decisions; shorter ones are acknowledgements.
	MinPrompt = 16
	// The most of a prompt and of the reply before it one extraction sends.
	promptBytes  = 8000
	contextRunes = 800
	// The most decisions one prompt adds, and the longest one may be.
	maxPerPrompt = 5
	maxRuleRunes = 300
	minQuote     = 3
	extractOut   = 1024
)

// Frictions: why a decision was earned. Only these are admitted.
const (
	frictionCorrection = "correction"
	frictionRegression = "regression"
	frictionSubtle     = "subtle"
)

// TurnEnded reads the turn's prompt for decisions in the background. Only
// Agent sessions teach (Ask and model-written prompts do not), one
// extraction per session runs at a time, and a prompt arriving while one
// runs is skipped rather than queued.
func (s *Service) TurnEnded(ctx context.Context, rec engine.TurnRecord) {
	if !learns(rec.Profile.Mode) || rec.Session.Temporary || rec.Goal || !s.enabled() {
		return
	}
	session := rec.Session.ID
	before := s.remember(session, rec.Text)
	prompt := strings.TrimSpace(rec.Prompt)
	project := projectRoot(rec.TurnContext)
	if project == "" || !admitPrompt(prompt) || !s.claim(session) {
		return
	}
	if !s.begin() {
		s.release(session)
		return
	}
	work, cancel := context.WithCancel(s.ctx)
	stop := context.AfterFunc(ctx, cancel)
	go func() {
		defer s.end()
		defer s.release(session)
		defer cancel()
		defer stop()
		s.extract(work, project, session, prompt, before)
	}()
}

// admitPrompt is the cheap gate before any model call: a prompt of at least
// MinPrompt characters that is not a slash command.
func admitPrompt(prompt string) bool {
	return !strings.HasPrefix(prompt, "/") && utf8.RuneCountInString(prompt) >= MinPrompt
}

// remember keeps a session's latest reply and returns the one before it,
// the context a short answer ("the second one") refers to.
func (s *Service) remember(session, reply string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, known := s.recent[session]
	if !known {
		s.order = append(s.order, session)
		if len(s.order) > recentSessions {
			delete(s.recent, s.order[0])
			s.order = s.order[1:]
		}
	}
	s.recent[session] = reply
	return before
}

func (s *Service) claim(session string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.extracting[session] {
		return false
	}
	s.extracting[session] = true
	return true
}

func (s *Service) release(session string) {
	s.mu.Lock()
	delete(s.extracting, session)
	s.mu.Unlock()
}

// extract asks the light model for the prompt's decisions, admits the ones
// that hold up and queues them. Failures leave nothing behind: a missed
// decision returns the next time the user has to say it.
func (s *Service) extract(ctx context.Context, project, session, prompt, before string) {
	args, _, err := s.call(ctx, request(extractSystem, extractInput(prompt, before), extractTool, extractSchema, extractOut))
	if err != nil {
		return
	}
	items, err := list(args, "decisions")
	if err != nil || len(items) == 0 {
		return
	}
	values := redact.Snapshot()
	id := BankID(project)
	var admitted []delta
	for _, item := range items {
		d, ok := admit(item, prompt)
		if !ok {
			continue
		}
		d.Rule, d.Quote = values.Strict(d.Rule), values.Strict(d.Quote)
		d.Session, d.At = session, s.now().UTC()
		admitted = append(admitted, d)
		if len(admitted) == maxPerPrompt {
			break
		}
	}
	if len(admitted) == 0 || ctx.Err() != nil {
		return
	}
	bank := s.bank(id)
	bank.mu.Lock()
	st := s.readState(id)
	if st.Project != project {
		st.Project = project
		if s.writeState(id, st) != nil {
			bank.mu.Unlock()
			return
		}
	}
	for _, d := range admitted {
		if s.enqueue(id, d) != nil {
			break
		}
	}
	bank.mu.Unlock()
	s.schedule(id, project, s.quiet)
	s.broadcast(project)
}

// extractInput is the user message of an extraction.
func extractInput(prompt, before string) string {
	var b strings.Builder
	if before = cleanContext(before, contextRunes); before != "" {
		b.WriteString("<assistant-before>" + before + "</assistant-before>\n")
	}
	if len(prompt) > promptBytes {
		prompt = strings.ToValidUTF8(prompt[:promptBytes], "")
	}
	b.WriteString("<user-prompt>\n" + prompt + "\n</user-prompt>")
	return b.String()
}

var (
	codeFence  = regexp.MustCompile("(?s)(```|~~~).*?(```|~~~)")
	whitespace = regexp.MustCompile(`\s+`)
)

// cleanContext flattens a reply to one line without its code, cut to max
// characters.
func cleanContext(text string, max int) string {
	text = codeFence.ReplaceAllString(text, "[code omitted]")
	text = strings.TrimSpace(whitespace.ReplaceAllString(text, " "))
	if utf8.RuneCountInString(text) > max {
		text = string([]rune(text)[:max])
	}
	return text
}

// admit is the host's gate on one extracted decision: a known area, a
// rule, a quote found word for word in the prompt (so nothing the assistant
// said becomes the user's decision), and friction (a correction, a
// regression, or a subtle rule). Decisions the user made once without
// friction are left to the code that already reflects them.
func admit(item map[string]any, prompt string) (delta, bool) {
	area := areaOf(field(item, "area", "kind"))
	if area == "" {
		return delta{}, false
	}
	raw := field(item, "rule", "statement", "text")
	if _, unsafe := scan.Strict(raw); unsafe {
		return delta{}, false
	}
	rule := oneLine(raw)
	if rule == "" {
		return delta{}, false
	}
	if utf8.RuneCountInString(rule) > maxRuleRunes {
		rule = strings.TrimSpace(string([]rune(rule)[:maxRuleRunes]))
	}
	quote, ok := quoted(field(item, "quote", "evidence"), prompt)
	if !ok {
		return delta{}, false
	}
	friction := frictionOf(item["friction"])
	if friction == "" {
		return delta{}, false
	}
	return delta{V: 1, Area: area, Rule: rule, Quote: quote, Friction: friction}, true
}

func areaOf(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, "_decision")
	value = strings.TrimSuffix(value, ".md")
	if validArea(value) {
		return value
	}
	return ""
}

// quoted finds quote in prompt, exactly or with runs of white space taken
// as one space, and returns it as the prompt has it.
func quoted(quote, prompt string) (string, bool) {
	quote = strings.Trim(strings.TrimSpace(quote), "\"'`\u201c\u201d\u2018\u2019")
	quote = strings.TrimSpace(quote)
	if utf8.RuneCountInString(quote) < minQuote {
		return "", false
	}
	if strings.Contains(prompt, quote) {
		return quote, true
	}
	flat := strings.TrimSpace(whitespace.ReplaceAllString(quote, " "))
	if strings.Contains(whitespace.ReplaceAllString(prompt, " "), flat) {
		return flat, true
	}
	return "", false
}

// frictionOf reads the friction a model gave: one of the three words, or
// omp's object of three flags, a regression outranking a correction.
func frictionOf(value any) string {
	switch v := value.(type) {
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "correction", "corrective", "corrected":
			return frictionCorrection
		case "regression", "regressed":
			return frictionRegression
		case "subtle":
			return frictionSubtle
		}
	case map[string]any:
		for _, pair := range [][2]string{{"regression", frictionRegression}, {"corrective", frictionCorrection}, {"correction", frictionCorrection}, {"subtle", frictionSubtle}} {
			if on, _ := v[pair[0]].(bool); on {
				return pair[1]
			}
		}
	}
	return ""
}

// oneLine makes a model's text one trimmed line without a list marker.
func oneLine(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, text)
	text = strings.TrimSpace(whitespace.ReplaceAllString(text, " "))
	return bulletText(text)
}
