package sensitive

import (
	"strings"
	"testing"
)

func TestLabel(t *testing.T) {
	cases := []struct {
		text, want string
	}{
		{"Password", whatPassword},
		{"New passwords", whatPassword},
		{"confirmNewPassword", whatPassword},
		{"user_passcode", whatPasscode},
		{"Enter PIN", whatPIN},
		{"PINs", whatPIN},
		{"pincode", whatPIN},
		{"CVV", whatCVV},
		{"Card number", whatCard},
		{"card numbers", whatCard},
		{"creditcard", whatCard},
		{"Account number", whatBank},
		{"Sort code", whatSortCode},
		{"SSN", whatSSN},
		{"Social Security", whatSSN},
		{"Passport", whatPassport},
		{"Driver's licence", whatLicence},
		{"One-time code", whatOTP},
		{"two-factor", whatOTP},
		{"Recovery phrase", whatRecovery},
		{"API key", whatKey},
		{"Client secret", whatSecret},
		{"Mot de passe", whatPassword},
		{"Passwort", whatPassword},
		{"パスワード", whatPassword},
		{"Pinned tabs", ""},
		{"Spinner", ""},
		{"Opinion poll", ""},
		{"Passing grade", ""},
		{"Account settings", ""},
		{"Add card", ""},
		{"com.github.pinta", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := Label(c.text); got != c.want {
			t.Errorf("Label(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestPlace(t *testing.T) {
	for _, s := range []string{"org.keepassxc.KeePassXC", "com.bitwarden.desktop", "1Password", "pinentry-qt",
		"org.gnome.seahorse.Application", "Enter your PIN", "Bank — Two-factor authentication", "card 4111 1111 1111 1111"} {
		if !Place(s) {
			t.Errorf("Place(%q) = false", s)
		}
	}
	// a long mixed-case id with digits is an app, not a token, as a place
	for _, s := range []string{"", "org.gnome.TextEditor", "firefox", "com.github.tchx84.Flatseal", "github.com/neur0map/Ryoku"} {
		if Place(s) {
			t.Errorf("Place(%q) = true", s)
		}
	}
}

func TestSecret(t *testing.T) {
	cases := []struct {
		text   string
		secret bool
	}{
		{"my password is swordfish", true},
		{"the PIN is four four two one", true},
		{"4111 1111 1111 1111", true},
		{"482913", true},
		{"482 913", true},
		{"48291375", true},
		{"sk-proj-abcdefghijklmnop", true},
		{"token ghp_aaaaBBBBccccDDDD1234", true},
		{"AKIAIOSFODNN7EXAMPLE", true},
		{"Xk9fQ2mZ7pL4wR8tY1vB3nC6", true},
		{"-----BEGIN OPENSSH PRIVATE KEY-----", true}, // rashin-reviewer:allow private-key - header-only rejection fixture
		{"ssn 123-45-6789", true},
		{"Ada Lovelace, 10 Downing St", false},
		{"in 2026", false},
		{"order 12", false},
		{"spin the wheel", false},
		{"a pinecone fell", false},
		{"call me at 0113 496 0000", false},
		{"card 4111 1111 1111 1112 is a typo", false},
		{"ship the release notes", false},
	}
	for _, c := range cases {
		if got := Secret(c.text); got != c.secret {
			t.Errorf("Secret(%q) = %v, want %v", c.text, got, c.secret)
		}
	}
}

func TestNote(t *testing.T) {
	for _, s := range []string{"my password is hunter2", "the visa reads 4111 1111 1111 1111", "my locker is 48 29 13",
		"my passport is in the drawer", "the bank card is blue"} {
		if !Note(s) {
			t.Errorf("Note(%q) = false", s)
		}
	}
	for _, s := range []string{"the exam is on friday", "the bins go out on tuesday", "room 1204", "the surprise is in june"} {
		if Note(s) {
			t.Errorf("Note(%q) = true", s)
		}
	}
}

func TestNumber(t *testing.T) {
	cases := []struct {
		text string
		want Kind
	}{
		{"my card is 4111 1111 1111 1111", Card},
		{"4242424242424242", Card},
		{"4111-1111-1111-1111 expires soon", Card},
		{"ssn 123-45-6789", ID},
		{"it is 123 45 6789", ID},
		{"NI number AB123456C", ID},
		{"AB 12 34 56 C", ID},
		{"DNI 12345678Z", ID},
		{"NIE X1234567L", ID},
		{"CPF 111.444.777-35", ID},
		{"aadhaar 2345 6789 0124", ID},
		{"sécu 1 85 05 78 006 084 91", ID},
		{"IBAN GB82 WEST 1234 5698 7654 32", Account},
		{"DE89370400440532013000", Account},
		{"my passport number is 123456789", ID},
		{"account number 12345678 at the bank", Account},
		{"card number 5555 5555 5555 4440", Card},
		{"the card number ends in 4242", None},
		{"x 2345 6789 0124 9", None},
		{"000-12-3456", None},
		{"666-12-3456", None},
		{"900-12-3456", None},
		{"123-00-6789", None},
		{"QQ123456C", None},
		{"GB123456A", None},
		{"12345678A", None},
		{"111.111.111-11", None},
		{"111.444.777-36", None},
		{"2345 6789 0123", None},
		{"card 4111 1111 1111 1112 is a typo", None},
		{"GB82 WEST 1234 5698 7654 33", None},
		{"my passport expires 2027-03-14", None},
		{"call me at 0113 496 0000", None},
		{"order 482913 ships monday", None},
		{"in 2026 we moved", None},
		{"", None},
	}
	for _, c := range cases {
		if got := Number(c.text); got != c.want {
			t.Errorf("Number(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestMask(t *testing.T) {
	cases := []struct {
		text, want string
	}{
		{"my card is 4111 1111 1111 1111 ok", "my card is •••• •••• •••• 1111 ok"},
		{"ssn 123-45-6789", "ssn •••-••-••••"},
		{"IBAN GB82 WEST 1234 5698 7654 32.", "IBAN •••• •••• •••• •••• •••• ••."},
		{"passport number 123456789 and card 4242424242424242", "passport number ••••••••• and card ••••••••••••4242"},
		{"the exam is on friday", "the exam is on friday"},
		{"order 482913", "order 482913"},
	}
	for _, c := range cases {
		got := Mask(c.text)
		if got != c.want {
			t.Errorf("Mask(%q) = %q, want %q", c.text, got, c.want)
		}
		if Number(got) != None {
			t.Errorf("Mask(%q) still shows a number: %q", c.text, got)
		}
	}
}

// Nothing a check reads ends up anywhere but its answer: Mask of text with
// no number returns the same string, byte for byte.
func TestMaskLeavesPlainTextAlone(t *testing.T) {
	for _, s := range []string{"naïve café 東京 👩‍👩‍👧", strings.Repeat("a", 1000), "1 2 3"} {
		if Mask(s) != s {
			t.Errorf("Mask changed %q", s)
		}
	}
}
