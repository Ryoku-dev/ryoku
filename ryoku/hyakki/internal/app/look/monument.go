package look

import (
	"gioui.org/layout"
)

// one shape for the claim every full page opens with, so the pages never drift apart
func Monument(gtx layout.Context, th *Theme, claim, proof string) layout.Dimensions {
	children := []layout.FlexChild{layout.Rigid(Paragraph(th, th.T.Sentence, th.R.Fg, claim))}
	if proof != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(Paragraph(th, th.T.Proof, th.R.Fg2, proof)),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
