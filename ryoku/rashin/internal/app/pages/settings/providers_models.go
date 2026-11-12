package settings

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// an open provider draws this many of its models at most; the search finds the rest
const modelRowsShown = 30

// the models an open provider row lists: asked for when the row opens and again when the provider's list
// changes, sorted once, matched once per query, never per frame
type modelList struct {
	search kit.Search
	again  kit.Plaque

	// what the provider said when the list was asked for, so a new listing asks again
	askedFor string
	loading  bool
	failed   string
	models   []protocol.ModelInfo

	query   string
	built   bool
	shown   []modelLine
	matches int
}

// one drawn row, its words made when the matches are
type modelLine struct {
	name, detail string
}

func (l *modelList) layout(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo) layout.Dimensions {
	th := e.Theme
	l.update(gtx, e, provider)
	children := make([]layout.FlexChild, 0, 2*len(l.shown)+8)
	children = append(children,
		layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Models"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, listSource(provider))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return look.Named(gtx, i18n.Tf("Search %s models", provider.Name), func(gtx layout.Context) layout.Dimensions {
						return l.search.Layout(gtx, th.Theme)
					})
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return l.again.Layout(gtx, th.Theme) }),
			)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
	)
	for _, line := range l.shown {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S1, Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.RawText(th.Theme, th.T.Body, th.R.Fg, line.name)),
					layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, line.detail)),
				)
			})
		}))
	}
	if note := l.note(); note != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, note)),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (l *modelList) update(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo) {
	l.search.Placeholder = i18n.T("Search models")
	l.again.Kind, l.again.Small, l.again.Label, l.again.Busy = kit.PlaqueTonal, true, i18n.T("Read the list again"), l.loading
	if l.again.Clicked(gtx) {
		l.ask(e, provider, true)
	} else if state := listState(provider); state != l.askedFor && !l.loading {
		l.ask(e, provider, false)
	}
	if query := strings.TrimSpace(l.search.Text()); query != l.query || !l.built {
		l.query = query
		l.match()
	}
}

// fresh has the daemon read the provider's own listing now, at most once a minute whoever asks
func (l *modelList) ask(e *env.Env, provider protocol.ProviderInfo, fresh bool) {
	l.askedFor, l.loading = listState(provider), true
	var result protocol.ModelsResult
	e.Call(protocol.MModelsList, protocol.ModelsParams{Provider: provider.ID, Fresh: fresh}, &result, func(err error) {
		l.loading = false
		if err != nil {
			l.failed = err.Error()
			return
		}
		l.failed, l.models = "", sortedModels(result.Models)
		l.built = false
	})
}

// what changes when the daemon reads a new listing or a model starts or stops answering
func listState(provider protocol.ProviderInfo) string {
	listed := ""
	if provider.Listed != nil {
		listed = provider.Listed.Format(time.RFC3339)
	}
	return fmt.Sprint(provider.Models, provider.Free, provider.Available, listed)
}

// what answers comes first, then what costs nothing, then Rashin's own picks before the rest of the listing
func sortedModels(models []protocol.ModelInfo) []protocol.ModelInfo {
	sorted := append([]protocol.ModelInfo(nil), models...)
	rank := func(m protocol.ModelInfo) int {
		rank := 0
		if !m.Available {
			rank += 4
		}
		if m.Tier == protocol.TierPaid {
			rank += 2
		}
		if m.Listed {
			rank++
		}
		return rank
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if left, right := rank(sorted[i]), rank(sorted[j]); left != right {
			return left < right
		}
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})
	return sorted
}

func (l *modelList) match() {
	l.built = true
	l.shown, l.matches = l.shown[:0], 0
	query := strings.ToLower(l.query)
	for _, m := range l.models {
		if query != "" && !strings.Contains(strings.ToLower(m.Name), query) && !strings.Contains(strings.ToLower(m.ID), query) {
			continue
		}
		l.matches++
		if len(l.shown) < modelRowsShown {
			l.shown = append(l.shown, modelLine{name: m.Name, detail: modelWords(m)})
		}
	}
}

func (l *modelList) note() string {
	switch {
	case l.failed != "":
		return l.failed
	case l.loading && len(l.models) == 0:
		return i18n.T("Reading the list.")
	case l.matches == 0 && l.query != "":
		return i18n.Tf("No model matches %s.", l.query)
	case l.matches > len(l.shown):
		return i18n.Tf("Showing %d of %s. Search to find the rest.", len(l.shown), look.Grouped(l.matches))
	}
	return ""
}

// where the list comes from, in a sentence under its title
func listSource(provider protocol.ProviderInfo) string {
	switch {
	case provider.Local:
		return i18n.T("What the server on this computer serves now.")
	case provider.Listed != nil:
		return i18n.Tf("From the provider's own list, read at %s. Auto, Fast and Smart use the free ones whose tool support is known.", provider.Listed.Local().Format("15:04"))
	case !provider.Configured:
		return i18n.T("Rashin's picks. Add a key and Rashin reads the provider's whole list.")
	}
	return i18n.T("Rashin's picks. The provider's own list has not been read yet.")
}

// the id, what it costs, how much it reads and what it can do, on the line under a model's name
func modelWords(m protocol.ModelInfo) string {
	words := []string{strings.TrimPrefix(m.ID, m.Provider+"/"), tierWord(m.Tier)}
	if m.Context > 0 {
		words = append(words, contextWords(m.Context))
	}
	switch {
	case !m.ToolsKnown:
		words = append(words, i18n.T("tools unknown"))
	case m.Tools:
		words = append(words, i18n.T("tools"))
	default:
		words = append(words, i18n.T("no tools"))
	}
	if m.Vision {
		words = append(words, i18n.T("reads pictures"))
	}
	if !m.Available {
		words = append(words, i18n.T("not available now"))
	}
	return strings.Join(words, ", ")
}

func tierWord(tier string) string {
	switch tier {
	case protocol.TierPaid:
		return i18n.T("paid")
	case protocol.TierCredits:
		return i18n.T("free credits")
	case protocol.TierLocal:
		return i18n.T("on this computer")
	}
	return i18n.T("free")
}

// 131072 reads as 131K, 1048576 as 1M
func contextWords(tokens int) string {
	if tokens >= 1_000_000 {
		return i18n.Tf("%sM context", strconv.FormatFloat(math.Round(float64(tokens)/100_000)/10, 'f', -1, 64))
	}
	return i18n.Tf("%dK context", tokens/1000)
}
