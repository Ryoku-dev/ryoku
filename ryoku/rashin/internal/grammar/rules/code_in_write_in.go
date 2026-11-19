package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type CodeInWriteIn struct{ expression expr.Expr }

func NewCodeInWriteIn() *CodeInWriteIn {
	return &CodeInWriteIn{expression: expr.NewSequenceExprWordSet([]string{"coded", "codes", "coding", "write", "writes", "written", "wrote", "program", "programmed", "programming"}).TWs().TAco("on").TWs().ThenWordSet([]string{"ada", "asm", "assembler", "assembly", "bash", "basic", "c", "clisp", "cobol", "cpp", "csharp", "css", "dart", "ecmascript", "fortran", "haskell", "html", "go", "golang", "java", "javascript", "js", "julia", "kotlin", "lisp", "oberon", "objc", "odin", "pascal", "perl", "php", "python", "ruby", "rust", "sh", "shell", "swift", "typescript", "zig", "zsh"})}
}
func (r *CodeInWriteIn) Expr() expr.Expr                  { return r.expression }
func (r *CodeInWriteIn) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CodeInWriteIn) Description() string {
	return "Corrects the wrong preposition `on` to `in` when referring to writing code."
}
func (r *CodeInWriteIn) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) != 5 {
		return nil
	}
	span := toks[2].Span
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("in", span.GetContent(src))}
	l.Message = "For writing code, the preposition should be `in` rather than `on`."
	return &l
}
