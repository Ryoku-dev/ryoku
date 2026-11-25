package chat

import "ryoku-rashin/internal/app/look"

// an answer is read line after line, so its lines stand further apart than proof text elsewhere
func readingStyle(th *look.Theme) look.TextStyle {
	style := th.T.Proof
	style.Line = 24
	return style
}

// code in a sentence is told apart by its face alone, set a step up from mono so it stands as tall as the words around it
func codeStyle(th *look.Theme) look.TextStyle {
	style := th.T.Mono
	style.Size = 13
	return style
}
