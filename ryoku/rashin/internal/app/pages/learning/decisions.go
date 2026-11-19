package learning

import (
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type decisionsSection struct {
	banks   []protocol.DecisionBank
	remove  map[string]*kit.Plaque
	pending uint64
	err     string
}

func (s *decisionsSection) load(e *env.Env) {
	s.pending++
	request := s.pending
	var result protocol.LearningDecisionsResult
	e.Call(protocol.MLearningDecisions, protocol.LearningDecisionsParams{}, &result, func(err error) {
		if request != s.pending {
			return
		}
		if err != nil {
			s.err = err.Error()
			return
		}
		s.banks, s.err = result.Banks, ""
	})
}

func (s *decisionsSection) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	if s.remove == nil {
		s.remove = make(map[string]*kit.Plaque)
	}
	rows := []look.Setting{}
	if s.err != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Could not read project decisions"), Detail: s.err})
	}
	for _, bank := range s.banks {
		bank := bank
		label := projectName(bank.Project)
		detail := bank.Project
		if bank.Queued > 0 {
			detail += " · " + i18n.Tf("%d waiting to be consolidated", bank.Queued)
		}
		if bank.Error != "" {
			detail += " · " + bank.Error
		}
		rows = append(rows, look.Setting{Title: label, Detail: detail})
		for _, area := range protocol.DecisionAreas {
			area := area
			text := bank.Architecture
			switch area {
			case protocol.DecisionProduct:
				text = bank.Product
			case protocol.DecisionStyle:
				text = bank.Style
			}
			name := decisionAreaName(area)
			lines := decisionLines(text)
			count := 0
			for _, line := range lines {
				if strings.HasPrefix(line, "- ") {
					count++
				}
			}
			rows = append(rows, look.Setting{Title: name, Detail: i18n.Tf("%d decisions", count)})
			heading := ""
			for index, line := range lines {
				if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
					heading = strings.TrimLeft(strings.TrimSpace(line), "# ")
					continue
				}
				if !strings.HasPrefix(line, "- ") {
					continue
				}
				line, heading, index := line, heading, index
				key := bank.Project + "\x00" + area + "\x00" + strconv.Itoa(index)
				button := s.remove[key]
				if button == nil {
					button = &kit.Plaque{}
					s.remove[key] = button
				}
				rows = append(rows, look.Setting{Title: line, Detail: heading, Control: func(gtx layout.Context) layout.Dimensions {
					if button.Clicked(gtx) {
						var updated protocol.DecisionBank
						e.Call(protocol.MLearningDecisionDelete, protocol.DecisionDeleteParams{Project: bank.Project, Area: area, Line: line}, &updated, func(err error) {
							if err != nil {
								e.Report(err)
								return
							}
							s.load(e)
							e.Notify(i18n.T("Decision removed."))
						})
					}
					button.Kind, button.Small, button.Label = kit.PlaqueDangerLine, true, i18n.T("Remove decision")
					return button.Layout(gtx, th.Theme)
				}})
			}
		}
	}
	if len(s.banks) == 0 && s.err == "" {
		rows = append(rows, look.Setting{Title: i18n.T("No project decisions yet"), Detail: i18n.T("Decisions Rashin learns from project work will appear here.")})
	}
	return look.SettingsSlab(gtx, th, rows)
}

func decisionLines(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return strings.Split(strings.TrimSpace(text), "\n")
}

func decisionAreaName(area string) string {
	switch area {
	case protocol.DecisionArchitecture:
		return i18n.T("Architecture")
	case protocol.DecisionProduct:
		return i18n.T("Product")
	case protocol.DecisionStyle:
		return i18n.T("Style")
	}
	return area
}
