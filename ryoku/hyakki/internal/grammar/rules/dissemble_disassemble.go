package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type DissembleDisassemble struct{ expression expr.Expr }

func NewDissembleDisassemble() *DissembleDisassemble {
	return &DissembleDisassemble{expression: expr.AsExpr(expr.NewWordSet([]string{"dissemble", "dissembled", "dissembles", "dissembling", "dissembler", "dissemblers", "dissembly", "dissemblies"}))}
}
func (r *DissembleDisassemble) Expr() expr.Expr { return r.expression }
func (r *DissembleDisassemble) Description() string {
	return "Tries to detect `dissemble` used instead of `disassemble` by mistake."
}
func (r *DissembleDisassemble) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DissembleDisassemble) Unit() lint.DocumentIterator      { return lint.Sentence{} }
func (r *DissembleDisassemble) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}

var dissembleDisassembleTriggers = []string{"128", "32bit", "64", "64bit", "68k", "aircraft", "algorithm", "analyse", "analysis", "analyze", "arch", "architecture", "arduino", "asm", "assemble", "assembled", "assembler", "assembly", "atari", "binary", "buffer", "bypass", "byte", "bytes", "cable", "cables", "cheat", "code", "compiler", "component", "components", "computer", "contraption", "controller", "controls", "core", "crash", "debug", "debugger", "decompile", "decompiled", "decompiler", "dex", "disassemble", "disassembler", "disassembly", "dismantle", "dll", "drive", "dump", "dumped", "electronic", "electronics", "enclosure", "engine", "engineer", "engineered", "engineering", "exe", "executable", "executables", "execute", "exploit", "exploiting", "firewall", "firmware", "function", "functions", "furniture", "game", "games", "gdb", "ghidra", "hack", "hacking", "hardware", "hex", "ida", "instruction", "instructions", "kit", "lib", "library", "load", "logic", "metadata", "motor", "nintendo", "opcode", "opcodes", "operation", "operations", "oscilloscope", "parameter", "parameters", "penetration", "pic", "processor", "protocol", "pseudocode", "register", "reverse", "rom", "runtime", "sata", "save", "sector", "sega", "server", "smali", "software", "stack", "switches", "toolkit", "usb", "windbg", "wires", "x64", "x86"}

func (r *DissembleDisassemble) MatchToLintWithContext(t []doc.Token, s []rune, ctx *lint.MatchContext) *lint.Lint {
	found := false
	if ctx != nil {
		for _, tokens := range [][]doc.Token{ctx.Before, ctx.After} {
			for _, tok := range tokens {
				if text.EqAnyIgnoreASCIICaseStr(tok.GetCh(s), dissembleDisassembleTriggers) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	if !found || len(t) == 0 {
		return nil
	}
	original := t[0].GetCh(s)
	if len(original) < 5 {
		return nil
	}
	word := make([]rune, 0, len(original)+2)
	word = append(word, original[:2]...)
	word = append(word, original[2], rune(byte(original[4])-4))
	word = append(word, original[2:]...)
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith(word)}, Message: "Did you confuse `dissemble` for `disassemble?", Priority: 127}
}
