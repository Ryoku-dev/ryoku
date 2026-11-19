package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCodeInWriteIn_FixCodedOnAda(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Prompt generator based on Bernoulli. Coded on Ada language.", NewCodeInWriteIn(), "Prompt generator based on Bernoulli. Coded in Ada language.")
}

func TestCodeInWriteIn_FixCodedOnCpp(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A network function simulation project coded on cpp.", NewCodeInWriteIn(), "A network function simulation project coded in cpp.")
}

func TestCodeInWriteIn_FixCodedOnPhp(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A book keeping application coded on PHP to maintain an inventory of books and a history of users reading them.", NewCodeInWriteIn(), "A book keeping application coded in PHP to maintain an inventory of books and a history of users reading them.")
}

func TestCodeInWriteIn_FixCodingOnObjcCodingOnSwift(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Import XibView.h and XibView.m if you're coding on objc or XibView.swift if you're coding on Swift.", NewCodeInWriteIn(), "Import XibView.h and XibView.m if you're coding in objc or XibView.swift if you're coding in Swift.")
}

func TestCodeInWriteIn_FixCodingOnPython(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Was coding on Python via VS Code and started getting this error message", NewCodeInWriteIn(), "Was coding in Python via VS Code and started getting this error message")
}

func TestCodeInWriteIn_FixProgramOnAssembler(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Learn to program on assembler dividing numbers", NewCodeInWriteIn(), "Learn to program in assembler dividing numbers")
}

func TestCodeInWriteIn_DontFlagProgramOnAssemblyLevel(t *testing.T) {
	t.Skip("Ignored upstream: Known false positive - 'on assembly level' is a valid phrase")
	linttest.AssertNoLints(t, "This code is a tool which can be used to program on assembly level using natural language.", NewCodeInWriteIn())
}

func TestCodeInWriteIn_FixProgramOnBasic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Write this program on BASIC first:", NewCodeInWriteIn(), "Write this program in BASIC first:")
}

func TestCodeInWriteIn_FixProgramOnPython(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Performance Issue writing a long program on python", NewCodeInWriteIn(), "Performance Issue writing a long program in python")
}

func TestCodeInWriteIn_FixProgrammingOnC(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My complain about lark is that I cannot use it when programming on C or C++, Java, Javascript, etc.", NewCodeInWriteIn(), "My complain about lark is that I cannot use it when programming in C or C++, Java, Javascript, etc.")
}

func TestCodeInWriteIn_DontFlagProgrammingOnEcmascript(t *testing.T) {
	t.Skip("Ignored upstream: 'Platforms' is separated from 'ECMAScript' due to being part of a list.")
	linttest.AssertNoLints(t, "Secure Distributed Programming on ECMAScript 5 + HTML5 Platforms", NewCodeInWriteIn())
}

func TestCodeInWriteIn_FixProgrammingOnHaskell(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is material for an introduction to functional programming on Haskell, with a focus on monads.", NewCodeInWriteIn(), "This is material for an introduction to functional programming in Haskell, with a focus on monads.")
}

func TestCodeInWriteIn_FixProgrammingOnJavascript(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Massively parallel GPU programming on JavaScript, simple and clean.", NewCodeInWriteIn(), "Massively parallel GPU programming in JavaScript, simple and clean.")
}

func TestCodeInWriteIn_DontFlagProgramsOnJava(t *testing.T) {
	linttest.AssertNoLints(t, "Notes and Programs on Java.", NewCodeInWriteIn())
}

func TestCodeInWriteIn_FixProgrammingOnRuby(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Deep Learning Programming on Ruby by Kenta Murata & Yusaku Hatanaka", NewCodeInWriteIn(), "Deep Learning Programming in Ruby by Kenta Murata & Yusaku Hatanaka")
}

func TestCodeInWriteIn_FixProgrammingOnRust(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Programming on Rust is like solving riddles, and I love riddles.", NewCodeInWriteIn(), "Programming in Rust is like solving riddles, and I love riddles.")
}

func TestCodeInWriteIn_FixWrittenOnAsm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Contracts are written on ASM-like language.", NewCodeInWriteIn(), "Contracts are written in ASM-like language.")
}

func TestCodeInWriteIn_FixWrittenOnAssembler(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Electronic design automation system written on Assembler (FASM).", NewCodeInWriteIn(), "Electronic design automation system written in Assembler (FASM).")
}

func TestCodeInWriteIn_FixWrittenOnBash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A collection of simple and different tools written on Bash that simplify work.", NewCodeInWriteIn(), "A collection of simple and different tools written in Bash that simplify work.")
}

func TestCodeInWriteIn_FixWrittenOnCsharp(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is a small Windows application written on csharp and wpf that requires Stockfish to to assist in playing on chess.com and lichess.org.", NewCodeInWriteIn(), "It is a small Windows application written in csharp and wpf that requires Stockfish to to assist in playing on chess.com and lichess.org.")
}

func TestCodeInWriteIn_FixWrittenOnDart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a command-line app written on dart language for flutter applications that will help you to generate some boilerplate code", NewCodeInWriteIn(), "This is a command-line app written in dart language for flutter applications that will help you to generate some boilerplate code")
}

func TestCodeInWriteIn_FixWrittenOnFortran(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Simple Telegram Bot written on FORTRAN for generating LaTeX pictures in private messages and inline mode", NewCodeInWriteIn(), "Simple Telegram Bot written in FORTRAN for generating LaTeX pictures in private messages and inline mode")
}

func TestCodeInWriteIn_FixWrittenOnGo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Template for a typical module written on Go.", NewCodeInWriteIn(), "Template for a typical module written in Go.")
}

func TestCodeInWriteIn_FixWrittenOnGolang(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Discovery library for Tarantool 3.0 written on Golang", NewCodeInWriteIn(), "Discovery library for Tarantool 3.0 written in Golang")
}

func TestCodeInWriteIn_FixWrittenOnHtml(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Developer's Life is a simple but exciting web-game written on HTML, CSS and JS", NewCodeInWriteIn(), "Developer's Life is a simple but exciting web-game written in HTML, CSS and JS")
}

func TestCodeInWriteIn_FixWrittenOnKotlin(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As on the image above decompiled java bytecode of apps written on kotlin doesn't look so good.", NewCodeInWriteIn(), "As on the image above decompiled java bytecode of apps written in kotlin doesn't look so good.")
}

func TestCodeInWriteIn_FixWrittenOnLisp(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Use flexible configuration files written on Lisp, provided by hygienic user bindings", NewCodeInWriteIn(), "Use flexible configuration files written in Lisp, provided by hygienic user bindings")
}

func TestCodeInWriteIn_FixWrittenOnPascal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Project allows converting program file written on Pascal into Java bytecode.", NewCodeInWriteIn(), "Project allows converting program file written in Pascal into Java bytecode.")
}

func TestCodeInWriteIn_FixWrittenOnPerl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "exiftool is written on Perl!", NewCodeInWriteIn(), "exiftool is written in Perl!")
}

func TestCodeInWriteIn_FixWrittenOnZig(t *testing.T) {
	linttest.AssertSuggestionResult(t, "XML parser written on Zig - by snektron.", NewCodeInWriteIn(), "XML parser written in Zig - by snektron.")
}

func TestCodeInWriteIn_FixWroteOnJulia(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm running into the same problem, and same illegal instruction, with some code that I wrote on Julia 1.0 (official binaries).", NewCodeInWriteIn(), "I'm running into the same problem, and same illegal instruction, with some code that I wrote in Julia 1.0 (official binaries).")
}
