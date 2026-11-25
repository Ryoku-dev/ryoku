package tts

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalize(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"emphasis", "**Bold** and _italic_ and ~~gone~~ text", "Bold and italic and gone text."},
		{"heading", "# Title\nSome text", "Title. Some text."},
		{"lists", "- one\n* two\n1. three\n- [x] four", "one. two. three. four."},
		{"link", "See [the docs](https://example.com/docs) now.", "See the docs now."},
		{"image", "![a red fox](fox.png)", "a red fox."},
		{"bare url", "Visit https://www.example.com/path?q=1.", "Visit example.com."},
		{"autolink", "Use <https://go.dev/doc>.", "Use go.dev."},
		{"inline code", "Run `go_test()` first.", "Run go test first."},
		{"file name", "Open `config.json` now.", "Open config dot json now."},
		{"long code", "Try `for i := range items { process(items[i]) }` here.", "Try here."},
		{"fenced code", "Code:\n```go\nfmt.Println(1)\n```\nDone.", "Code. Done."},
		{"unclosed fence", "Hi\n~~~\ncode forever", "Hi."},
		{"inline math", `Area is $\pi r^2$.`, "Area is pi r squared."},
		{"display math", `$$\frac{a}{b} = c$$`, "a over b equals c."},
		{"paren math", `\(x \leq 5\)`, "x is less than or equal to 5."},
		{"bracket math", `\[\sqrt{x+1} \times 3\]`, "the square root of x plus 1 times 3."},
		{"power", `$e^{i\pi} + 1 = 0$`, "e to the power of i pi plus 1 equals 0."},
		{"subscript", `$x_{n} - x_1$`, "x sub n minus x sub 1."},
		{"text in math", `$\text{speed} = d/t$`, "speed equals d over t."},
		{"money", "It costs $5 or $6.50.", "It costs 5 dollars or 6.50 dollars."},
		{"table", "| a | b |\n|---|:---:|\n| 1 | 2 |", "a, b. 1, 2."},
		{"html and emoji", "<b>Hi</b> 👋 there ✨", "Hi there."},
		{"quote", "> quoted line\n> more", "quoted line more."},
		{"arrow", "A -> B => C", "A to B to C."},
		{"rule", "Above\n\n---\n\nBelow", "Above. Below."},
		{"stops", "Wait... what?!", "Wait. what?"},
		{"clause end", "Lists, like these:", "Lists, like these."},
		{"time kept", "It's 3:45.", "It's 3:45."},
		{"empty", " \n```\nx\n```\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Normalize(c.in); got != c.want {
				t.Fatalf("Normalize(%q)\n got %q\nwant %q", c.in, got, c.want)
			}
		})
	}
}

func TestSentences(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Hello there. How are you? Fine!", []string{"Hello there.", "How are you?", "Fine!"}},
		{"Mr. Smith is here. Yes.", []string{"Mr. Smith is here.", "Yes."}},
		{"Pi is 3.14 exactly. Next one.", []string{"Pi is 3.14 exactly.", "Next one."}},
		{"Use it e.g. this way. Done.", []string{"Use it e.g. this way.", "Done."}},
		{"J. R. wrote it. Then left.", []string{"J. R. wrote it.", "Then left."}},
		{`He said "go." Then went.`, []string{`He said "go."`, "Then went."}},
		{"no stop at the end", []string{"no stop at the end"}},
		{"...", nil},
		{"", nil},
	}
	for _, c := range cases {
		if got := Sentences(c.in); !slices.Equal(got, c.want) {
			t.Fatalf("Sentences(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	long := strings.Repeat("this clause runs on, ", 40) + "and ends."
	got := Sentences(long)
	if len(got) < 3 {
		t.Fatalf("a %d-rune sentence split into %d", utf8.RuneCountInString(long), len(got))
	}
	for _, s := range got {
		if n := utf8.RuneCountInString(s); n > maxSentenceRunes || n == 0 {
			t.Fatalf("piece of %d runes: %q", n, s)
		}
	}
	if strings.Join(got, " ") != strings.TrimSpace(long) {
		t.Fatalf("splitting lost text:\n%q", strings.Join(got, " "))
	}
}
