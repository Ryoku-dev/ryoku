package note

import "testing"

func TestParse(t *testing.T) {
	for _, c := range []struct {
		text string
		want Command
		ok   bool
	}{
		{"forget that", Command{Kind: Forget}, true},
		{"Okay, forget it.", Command{Kind: Forget}, true},
		{"forget that I live in Lisbon", Command{Kind: Forget, Arg: "I live in Lisbon"}, true},
		{"forget what you know about my sister", Command{Kind: Forget, Arg: "my sister"}, true},
		{"forget my gym schedule please", Command{Kind: Forget, Arg: "my gym schedule"}, true},
		{"forget everything", Command{}, false},
		{"don't forget that the bins go out on Tuesday", Command{}, false},
		{"do not forget the milk", Command{}, false},
		{"no, I meant Porto", Command{Kind: Correct, To: "Porto"}, true},
		{"actually I meant I live in Porto.", Command{Kind: Correct, To: "I live in Porto"}, true},
		{"correct Lisbon to Porto", Command{Kind: Correct, From: "Lisbon", To: "Porto"}, true},
		{`correct "tea" to "coffee"`, Command{Kind: Correct, From: "tea", To: "coffee"}, true},
		{"correct tea to tea", Command{}, false},
		{"what do you know about me", Command{Kind: Know}, true},
		{"what do you remember about my sister?", Command{Kind: Know, Arg: "my sister"}, true},
		{"what did I eat yesterday", Command{}, false},
		{"change the size to bigger", Command{}, false},
		{"remember that I live in Lisbon", Command{}, false},
		{"", Command{}, false},
	} {
		got, ok := Parse(c.text)
		if ok != c.ok || got != c.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v, %v", c.text, got, ok, c.want, c.ok)
		}
	}
}

func TestWritesNeedAPress(t *testing.T) {
	if !(Command{Kind: Forget}).Writes() || !(Command{Kind: Correct}).Writes() || (Command{Kind: Know}).Writes() {
		t.Fatal("Writes")
	}
}

func TestSplice(t *testing.T) {
	for _, c := range []struct {
		note, to, want string
		ok             bool
	}{
		{"I live in Lisbon", "Porto", "I live in Porto", true},
		{"I live in Lisbon.", "Porto", "I live in Porto.", true},
		{"My sister is called Ana", "Ines", "My sister is called Ines", true},
		{"I live in Lisbon", "I live in Porto", "I live in Porto", true},
		{"I live in Lisbon", "I live in Lisbon", "", false},
		{"Lisbon", "Porto and Braga", "Porto and Braga", true},
		{"", "Porto", "", false},
	} {
		got, ok := Splice(c.note, c.to)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("Splice(%q, %q) = %q, %v", c.note, c.to, got, ok)
		}
	}
}

func TestReplace(t *testing.T) {
	if got, ok := Replace("I drink Tea every morning, tea!", "tea", "coffee"); !ok || got != "I drink coffee every morning, coffee!" {
		t.Fatalf("Replace = %q, %v", got, ok)
	}
	if _, ok := Replace("I drink tea", "juice", "coffee"); ok {
		t.Fatal("replaced what was not there")
	}
}
