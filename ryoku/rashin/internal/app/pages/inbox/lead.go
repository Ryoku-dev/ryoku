package inbox

import (
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
)

// the sentence that says how things stand, and one line of proof under it
func layoutLead(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, needs int) layout.Dimensions {
	working := len(teamsui.Working(snapshot))
	sentence := i18n.T("All quiet.")
	switch {
	case needs == 1:
		sentence = i18n.T("One thing needs you.")
	case needs > 1:
		sentence = i18n.Tf("%s things need you.", look.CountStart(needs))
	case working > 0:
		sentence = i18n.T("Your teams are at work.")
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Sentence, th.R.Fg, sentence)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Proof, th.R.Fg2, leadProof(gtx.Now, snapshot, working))),
	)
}

// the newest report, how many work, and when a routine next runs, each a sentence of its own
func leadProof(now time.Time, snapshot state.Snapshot, working int) string {
	if len(snapshot.Teams) == 0 {
		return i18n.T("A team is a lead and its members, with one purpose, working in a folder of their own while you do something else.")
	}
	var sentences []string
	if task, ok := teamsui.NewestReport(snapshot, ""); ok {
		team := teamsui.TeamName(snapshot.Teams, task.Team)
		sentences = append(sentences, i18n.Tf("%s sent “%s” %s.", team, reportTitle(task), look.Ago(now, task.Report.At)))
	}
	switch {
	case working == 1:
		sentences = append(sentences, i18n.T("One task is working."))
	case working > 1:
		sentences = append(sentences, i18n.Tf("%d tasks are working.", working))
	}
	if runs := teamsui.Upcoming(snapshot, ""); len(runs) > 0 {
		next := runs[0]
		sentences = append(sentences, i18n.Tf("%s runs %s.", teamsui.TeamName(snapshot.Teams, next.Team), look.When(now, *next.NextRun)))
	}
	if len(sentences) == 0 {
		return i18n.T("Give a team a task and its report lands here.")
	}
	return strings.Join(sentences, " ")
}
