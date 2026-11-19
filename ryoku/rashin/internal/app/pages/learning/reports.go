package learning

import (
	"strconv"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type reportsSection struct {
	reports  protocol.LearningGrievancesResult
	clear    map[int64]*kit.Plaque
	clearAll kit.Plaque
	pending  uint64
	err      string
}

func (s *reportsSection) load(e *env.Env) {
	s.pending++
	request := s.pending
	var result protocol.LearningGrievancesResult
	e.Call(protocol.MLearningGrievances, protocol.LearningGrievancesParams{}, &result, func(err error) {
		if request != s.pending {
			return
		}
		if err != nil {
			s.err = err.Error()
			return
		}
		s.reports, s.err = result, ""
	})
}

func (s *reportsSection) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	if s.clear == nil {
		s.clear = make(map[int64]*kit.Plaque)
	}
	rows := []look.Setting{}
	if s.err != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Could not read tool reports"), Detail: s.err})
	}
	if s.reports.Total > 0 {
		rows = append(rows, look.Setting{Title: i18n.Tf("%d tool reports", s.reports.Total), Detail: i18n.T("These reports stay on this computer. Nothing is uploaded."), Control: func(gtx layout.Context) layout.Dimensions {
			if s.clearAll.Clicked(gtx) {
				s.remove(e, nil)
			}
			s.clearAll.Kind, s.clearAll.Small, s.clearAll.Label = kit.PlaqueDangerLine, true, i18n.T("Clear all reports")
			return s.clearAll.Layout(gtx, th.Theme)
		}})
	}
	for _, report := range s.reports.Grievances {
		report := report
		button := s.clear[report.ID]
		if button == nil {
			button = &kit.Plaque{}
			s.clear[report.ID] = button
		}
		detail := report.Report
		if report.Source != "" {
			detail += " · " + report.Source
		}
		if report.Model != "" {
			detail += " · " + report.Model
		}
		if report.Session != "" {
			detail += " · " + report.Session
		}
		if !report.At.IsZero() {
			detail += " · " + report.At.Local().Format("2 Jan 2006 15:04")
		}
		rows = append(rows, look.Setting{Title: report.Tool + " #" + strconv.FormatInt(report.ID, 10), Detail: detail, Below: true, Control: func(gtx layout.Context) layout.Dimensions {
			if button.Clicked(gtx) {
				s.remove(e, []int64{report.ID})
			}
			button.Kind, button.Small, button.Label = kit.PlaqueDangerLine, true, i18n.T("Clear report")
			return button.Layout(gtx, th.Theme)
		}})
	}
	if s.reports.Total == 0 && s.err == "" {
		rows = append(rows, look.Setting{Title: i18n.T("No tool reports yet"), Detail: i18n.T("If a tool misbehaves, Rashin can keep a private report for you to review.")})
	}
	return look.SettingsSlab(gtx, th, rows)
}

func (s *reportsSection) remove(e *env.Env, ids []int64) {
	e.Call(protocol.MLearningGrievancesClear, protocol.GrievancesClearParams{IDs: ids}, new(protocol.Empty), func(err error) {
		if err != nil {
			e.Report(err)
			return
		}
		s.load(e)
		e.Notify(i18n.T("Tool reports cleared."))
	})
}
