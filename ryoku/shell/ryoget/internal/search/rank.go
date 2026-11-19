package search

import (
	"html"
	"math"
	"net/url"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

// Query is one normalized search request.
type Query struct {
	Text string
	// lower case, keys the source tables ("pt-br")
	Lang string
	// for Accept-Language ("pt-BR")
	Locale  string
	suggest bool
	norm    string
	words   []string
	terms   []term
}

// term is a query word that counts towards relevance; short words count half.
type term struct {
	word   string
	stem   string
	weight float64
}

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "for": true, "to": true, "in": true, "on": true, "at": true,
	"and": true, "or": true, "with": true, "without": true, "how": true, "what": true, "why": true, "is": true,
	"are": true, "do": true, "does": true, "i": true, "my": true, "vs": true, "by": true, "from": true, "it": true,
}

func newQuery(text, lang string) Query {
	text = strings.Join(strings.Fields(text), " ")
	q := Query{Text: text, Lang: normLang(lang), Locale: normLocale(lang), norm: strings.ToLower(text), words: words(text)}
	for _, w := range q.words {
		if !stopwords[w] {
			q.terms = append(q.terms, newTerm(w))
		}
	}
	if len(q.terms) == 0 {
		for _, w := range q.words {
			q.terms = append(q.terms, newTerm(w))
		}
	}
	return q
}

func newTerm(w string) term {
	weight := 1.0
	if utf8.RuneCountInString(w) < 3 {
		weight = 0.5
	}
	return term{word: w, stem: stem(w), weight: weight}
}

var suffixes = []string{"ings", "ing", "ers", "ies", "ied", "ed", "er", "es", "ly", "s", "e"}

// stem cuts one common English ending so "sharing", "shared" and "share" meet.
func stem(w string) string {
	for _, s := range suffixes {
		if strings.HasSuffix(w, s) && utf8.RuneCountInString(w)-utf8.RuneCountInString(s) >= 3 {
			return strings.TrimSuffix(w, s)
		}
	}
	return w
}

// matches is lenient on purpose: same stem, a longer word starting with the
// term ("flicker" in "flickering"), or a compound holding it ("xwayland").
func (t term) matches(tok string) bool {
	n := utf8.RuneCountInString(t.word)
	return tok == t.word ||
		stem(tok) == t.stem ||
		n >= 4 && strings.HasPrefix(tok, t.word) ||
		n >= 5 && strings.Contains(tok, t.word)
}

// covers returns the weighted share of terms found in toks, and which.
// initials lets an acronym term ("obs") match a spelled-out title.
func (q Query) covers(toks []string, initials string, skip []bool) (float64, []bool) {
	hit := make([]bool, len(q.terms))
	var got, total float64
	for i, t := range q.terms {
		total += t.weight
		if skip != nil && skip[i] {
			continue
		}
		match := len(t.word) >= 3 && len(t.word) <= 5 && strings.Contains(initials, t.word)
		for _, tok := range toks {
			if match {
				break
			}
			match = t.matches(tok)
		}
		if match {
			hit[i] = true
			got += t.weight
		}
	}
	if total == 0 {
		return 0, hit
	}
	return got / total, hit
}

// relevance scores how well an item answers the query: the title's term
// coverage, plus half the coverage the snippet and site add on top.
func (q Query) relevance(it Item) (title, rel float64) {
	toks := words(it.Title)
	var initials strings.Builder
	for _, tok := range toks {
		r, _ := utf8.DecodeRuneInString(tok)
		initials.WriteRune(r)
	}
	title, hit := q.covers(toks, initials.String(), nil)
	extra, _ := q.covers(words(it.Snippet+" "+it.Site), "", hit)
	return title, title + 0.5*extra
}

// longest is the most specific single word, for APIs that match one term.
func (q Query) longest() string {
	best := ""
	for _, w := range q.words {
		if utf8.RuneCountInString(w) > utf8.RuneCountInString(best) {
			best = w
		}
	}
	return best
}

// hasAll reports whether every query word occurs in s.
func (q Query) hasAll(s string) bool {
	s = strings.ToLower(s)
	for _, w := range q.words {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}

func words(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '+' && r != '#'
	})
	seen := map[string]bool{}
	out := f[:0]
	for _, w := range f {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// rank orders one source's items: the source's own relevance order stays the
// backbone, query-term matches and popularity nudge it, duplicates go.
func rank(items []Item, q Query) []Item {
	for i := range items {
		it := &items[i]
		it.tcov, it.rel = q.relevance(*it)
		it.score = 1/(1+0.25*float64(i)) + 0.6*it.tcov + 0.5*(it.rel-it.tcov) + 0.25*clamp01(it.pop)
		title := strings.ToLower(it.Title)
		if title == q.norm || strings.TrimSuffix(title, "/") == q.norm {
			it.score += 0.4
		}
		if it.Meta == nil {
			it.Meta = map[string]any{}
		}
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].score > items[b].score })
	seen := map[string]bool{}
	out := items[:0]
	for _, it := range items {
		if k := normURL(it.URL); k != "" {
			if seen[k] {
				continue
			}
			seen[k] = true
		}
		out = append(out, it)
	}
	return out
}

// gate is how much an item must match the query to show in the All scope.
type gate int

const (
	// web and wiki sources already rank by full text; a term in the snippet will do
	gateLenient gate = iota
	// discussions, videos, repos and apps: a clear match across title and snippet
	gateMedium
	// code docs and package names: the title itself must carry the query
	gateStrict
	// instant answers are computed for the query
	gateNone
)

func (g gate) pass(it Item) bool {
	switch g {
	case gateLenient:
		return it.rel >= 0.25
	case gateMedium:
		return it.rel >= 0.5
	case gateStrict:
		return it.tcov >= 0.5
	}
	return true
}

// sectionRank orders All-scope sections: the mean relevance of a section's
// best two rows in quarter steps, so only clear differences reorder sources.
func sectionRank(items []Item) int {
	if len(items) == 0 {
		return 0
	}
	best := make([]float64, len(items))
	for i, it := range items {
		best[i] = it.rel
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(best)))
	if len(best) > 2 {
		best = best[:2]
	}
	sum := 0.0
	for _, v := range best {
		sum += v
	}
	return 1 + int(math.Floor(clamp01(sum/float64(len(best)))*4))
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// popularity maps a count onto 0..1 where 10^decades reaches 1.
func popularity(n float64, decades float64) float64 {
	if n <= 0 {
		return 0
	}
	return clamp01(math.Log10(n+1) / decades)
}

// normURL is the dedupe identity of a link: scheme, www., mobile hosts,
// fragments, trailing slashes and tracking parameters do not make a new page.
func normURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.ToLower(raw)
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.Replace(host, ".m.wikipedia.org", ".wikipedia.org", 1)
	host = strings.TrimPrefix(host, "m.")
	vals := u.Query()
	for k := range vals {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || lk == "ref" || lk == "fbclid" || lk == "gclid" {
			vals.Del(k)
		}
	}
	path := strings.TrimRight(u.Path, "/")
	out := host + path
	if enc := vals.Encode(); enc != "" {
		out += "?" + enc
	}
	return out
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// clean turns an API snippet (entities, highlight tags, wiki markup leftovers)
// into one line of plain text.
func clean(s string) string {
	if strings.ContainsAny(s, "<&") {
		s = stripTags(s)
	}
	return strings.Join(strings.Fields(s), " ")
}

// plain collapses whitespace in text that is not markup (API descriptions,
// parsed DOM text), where a literal "<T>" must survive.
func plain(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func stripTags(s string) string {
	var b strings.Builder
	z := xhtml.NewTokenizer(strings.NewReader(s))
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return html.UnescapeString(b.String())
		case xhtml.TextToken:
			b.Write(z.Text())
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, _ := z.TagName()
			if string(name) == "br" || string(name) == "p" {
				b.WriteByte(' ')
			}
		}
	}
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	cut := strings.LastIndexByte(string(r), ' ')
	out := string(r)
	if cut > len(out)/2 {
		out = out[:cut]
	}
	return strings.TrimRight(out, " ,.;:") + "…"
}

var blockTags = map[string]bool{
	"p": true, "div": true, "pre": true, "li": true, "br": true, "blockquote": true, "tr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "ul": true, "ol": true, "table": true,
}

// htmlText renders an HTML fragment as readable plain text: paragraphs become
// blank-line separated, list items get a dash, code blocks keep their lines.
func htmlText(s string, max int) string {
	var b strings.Builder
	z := xhtml.NewTokenizer(strings.NewReader(s))
	pre := 0
	newline := func(n int) {
		cur := b.String()
		trail := len(cur) - len(strings.TrimRight(cur, "\n"))
		for ; trail < n && b.Len() > 0; trail++ {
			b.WriteByte('\n')
		}
	}
loop:
	for b.Len() < max*2 {
		switch z.Next() {
		case xhtml.ErrorToken:
			break loop
		case xhtml.TextToken:
			t := html.UnescapeString(string(z.Text()))
			if pre > 0 {
				b.WriteString(t)
				continue
			}
			t = strings.Join(strings.Fields(t), " ")
			if t == "" {
				continue
			}
			cur := b.String()
			if cur != "" && !strings.HasSuffix(cur, "\n") && !strings.HasSuffix(cur, " ") {
				b.WriteByte(' ')
			}
			b.WriteString(t)
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "pre" {
				pre++
			}
			if blockTags[tag] {
				if tag == "li" {
					newline(1)
					b.WriteString("- ")
				} else if tag == "br" {
					newline(1)
				} else {
					newline(2)
				}
			}
		case xhtml.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "pre" && pre > 0 {
				pre--
			}
			if blockTags[tag] && tag != "li" {
				newline(2)
			}
		}
	}
	return clip(strings.TrimSpace(b.String()), max)
}
