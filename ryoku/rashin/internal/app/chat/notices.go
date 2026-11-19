package chat

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

func (state *viewState) layoutNotice(gtx layout.Context, th *look.Theme, notice protocol.NoticeData, key string) layout.Dimensions {
	return layout.Inset{Left: th.Sp.S2, Right: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return state.selectable(gtx, th, key, th.T.Caption, th.R.Fg3, notice.Text)
	})
}

func (state *viewState) layoutCompaction(gtx layout.Context, th *look.Theme, tool *protocol.ToolData, key string) layout.Dimensions {
	if tool == nil {
		return layout.Dimensions{}
	}
	label := i18n.T("Making room in this conversation")
	if !toolRunning(tool) {
		label = i18n.T("Made room in this conversation")
		if tool.Status == protocol.ToolFailed {
			label = i18n.T("Could not make room in this conversation")
		}
	}
	if output := strings.TrimSpace(tool.Output); output != "" {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, label)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key, th.T.Caption, th.R.Fg3, output)
			}),
		)
	}
	return kit.Text(th.Theme, th.T.Caption, th.R.Fg3, label)(gtx)
}
