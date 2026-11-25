package consent

import (
	"testing"
)

func button(name string) Subject { return Subject{Role: "button", Name: name} }

func TestEveryRiskyPressIsTheirs(t *testing.T) {
	labels := []string{
		"Send", "Send message", "Post", "Publish now", "Pay", "Pay $49.00", "Buy", "Buy now", "Purchase",
		"Submit", "Submit form", "Delete", "Delete draft", "Remove", "Remove item", "Sign out", "Log out",
		"Logout", "Unsubscribe", "Confirm purchase", "Place order", "Check out", "Checkout", "Donate",
		"Subscribe", "Reply", "Share", "Accept all", "I agree", "Authorize app", "Cancel subscription",
		"Close account", "Opt out", "Sign up", "Create account", "Upload", "Transfer funds",
		"SEND", "  send  ", "send-it", "Payments", "deleteAll",
		"Envoyer", "Enviar", "Bezahlen", "Supprimer",
	}
	for _, verb := range []string{VerbClick, VerbSelect} {
		for _, label := range labels {
			s := button(label)
			if verb == VerbSelect {
				s = Subject{Role: "combobox", Name: label, Option: "One"}
			}
			if c, r := Classify(verb, s, ClaimNormal); c != Handoff || r != ReasonRiskyLabel {
				t.Errorf("%s %q = %v %s, want handoff risky-label", verb, label, c, r)
			}
		}
	}
	for _, label := range labels {
		s := Subject{Role: "link", Name: label}
		if c, _ := Classify(VerbClick, s, ClaimNormal); c != Handoff {
			t.Errorf("link %q = %v, want handoff", label, c)
		}
	}
}

func TestLabelsTheRulesCannotReadAreTheirs(t *testing.T) {
	for _, label := range []string{
		"", "   ", "→", "✓", "$49", "42", "…", "Отправить", "送信", "发送", "إرسال", "Löschen", "Envoyé",
		"確認", "Tiếp", "\u200b",
	} {
		for _, verb := range []string{VerbClick, VerbFill, VerbSelect} {
			role := map[string]string{VerbClick: "button", VerbFill: "textbox", VerbSelect: "combobox"}[verb]
			c, r := Classify(verb, Subject{Role: role, Name: label, Option: "One"}, ClaimNormal)
			if c != Handoff || r != ReasonUnreadable {
				t.Errorf("%s %q = %v %s, want handoff unreadable-label", verb, label, c, r)
			}
		}
	}
	// A readable select whose option cannot be read is still theirs.
	if c, r := Classify(VerbSelect, Subject{Role: "combobox", Name: "Size", Option: "大"}, ClaimNormal); c != Handoff || r != ReasonUnreadable {
		t.Errorf("unreadable option = %v %s", c, r)
	}
	if c, r := Classify(VerbSelect, Subject{Role: "combobox", Name: "Then", Option: "Delete everything"}, ClaimNormal); c != Handoff || r != ReasonRiskyLabel {
		t.Errorf("risky option = %v %s", c, r)
	}
}

func TestPrivateFieldsAreRefused(t *testing.T) {
	cases := []Subject{
		{Role: "textbox", Name: "Password"},
		{Role: "textbox", Name: "New password"},
		{Role: "textbox", Name: "Anything", InputType: "password"},
		{Role: "textbox", Name: "Anything", InputType: "PASSWORD"},
		{Role: "textbox", Name: "Card number"},
		{Role: "textbox", Name: "Credit card"},
		{Role: "textbox", Name: "CVV"},
		{Role: "textbox", Name: "Security code"},
		{Role: "textbox", Name: "Expiry date"},
		{Role: "textbox", Name: "Social security number"},
		{Role: "textbox", Name: "National ID"},
		{Role: "textbox", Name: "Passport"},
		{Role: "textbox", Name: "IBAN"},
		{Role: "textbox", Name: "PIN"},
		{Role: "textbox", Name: "One time code"},
		{Role: "textbox", Name: "Number", Autocomplete: "cc-number"},
		{Role: "textbox", Name: "Code", Autocomplete: "section-pay billing cc-csc"},
		{Role: "textbox", Name: "Code", Autocomplete: "one-time-code"},
		{Role: "textbox", Name: "Secret", Autocomplete: "current-password"},
		{Role: "textbox", Name: "Secret", Autocomplete: "new-password"},
	}
	for _, s := range cases {
		for _, verb := range []string{VerbFill, VerbClick, VerbSelect} {
			for _, claim := range []string{ClaimNormal, ClaimSensitive, ClaimCritical, ""} {
				if c, r := Classify(verb, s, claim); c != Refused || r != ReasonPrivate {
					t.Errorf("%s %+v claim %q = %v %s, want refused private-field", verb, s, claim, c, r)
				}
			}
		}
	}
}

func TestOtherRefusals(t *testing.T) {
	cases := []struct {
		s Subject
		r Reason
	}{
		{Subject{Role: "button", Name: "Choose file", InputType: "file"}, ReasonFileInput},
		{Subject{Role: "button", Name: "Next", OtherOrigin: true}, ReasonOtherOrigin},
		{Subject{Role: "button", Name: "Next", Hidden: true}, ReasonHidden},
		{Subject{Role: "textbox", Name: "Token", InputType: "hidden"}, ReasonHidden},
	}
	for _, tc := range cases {
		for _, verb := range []string{VerbClick, VerbFill, VerbSelect} {
			if c, r := Classify(verb, tc.s, ClaimNormal); c != Refused || r != tc.r {
				t.Errorf("%s %+v = %v %s, want refused %s", verb, tc.s, c, r, tc.r)
			}
		}
	}
}

func TestTheModelCanOnlyRaiseTheRisk(t *testing.T) {
	risky := []Subject{button("Send"), button("Delete"), {Role: "button", Name: "Go", Submits: true}}
	for _, s := range risky {
		for _, claim := range []string{ClaimNormal, ClaimSensitive, ClaimCritical, "safe", "low", ""} {
			if c, _ := Classify(VerbClick, s, claim); c != Handoff {
				t.Errorf("%+v claimed %q = %v, want handoff", s, claim, c)
			}
		}
	}
	safe := Subject{Role: "link", Name: "Next page"}
	if c, r := Classify(VerbClick, safe, ClaimNormal); c != Safe || r != ReasonSafe {
		t.Fatalf("ordinary link = %v %s", c, r)
	}
	for _, claim := range []string{ClaimSensitive, ClaimCritical, "", "safe", "Normal", "none"} {
		if c, r := Classify(VerbClick, safe, claim); c != Handoff || r != ReasonClaim {
			t.Errorf("ordinary link claimed %q = %v %s, want handoff model-claim", claim, c, r)
		}
	}
	for _, verb := range []string{VerbScroll, VerbWait} {
		if c, _ := Classify(verb, Subject{}, ClaimCritical); c != Handoff {
			t.Errorf("%s claimed critical = %v, want handoff", verb, c)
		}
	}
}

func TestSafeSteps(t *testing.T) {
	cases := []struct {
		verb string
		s    Subject
	}{
		{VerbFill, Subject{Role: "textbox", Name: "Name"}},
		{VerbFill, Subject{Role: "textbox", Name: "Message"}},
		{VerbFill, Subject{Role: "textbox", Name: "Email", InputType: "email"}},
		{VerbFill, Subject{Role: "searchbox", Name: "Search", InputType: "search"}},
		{VerbFill, Subject{Role: "textbox", Name: "Street", PaymentForm: true}},
		{VerbClick, Subject{Role: "link", Name: "Contact us"}},
		{VerbClick, Subject{Role: "tab", Name: "Details"}},
		{VerbClick, Subject{Role: "checkbox", Name: "Gift wrap"}},
		{VerbClick, Subject{Role: "Button", Name: "Next"}},
		{VerbSelect, Subject{Role: "combobox", Name: "Country", Option: "Japan"}},
		{VerbScroll, Subject{}},
		{VerbWait, Subject{}},
	}
	for _, tc := range cases {
		if c, r := Classify(tc.verb, tc.s, ClaimNormal); c != Safe || r != ReasonSafe {
			t.Errorf("%s %+v = %v %s, want safe", tc.verb, tc.s, c, r)
		}
	}
}

func TestPressesThatLeaveThePageAreTheirs(t *testing.T) {
	cases := []struct {
		verb string
		s    Subject
		r    Reason
	}{
		{VerbClick, Subject{Role: "button", Name: "Go", Submits: true}, ReasonSubmits},
		{VerbClick, Subject{Role: "button", Name: "Go", InputType: "submit"}, ReasonSubmits},
		{VerbClick, Subject{Role: "button", Name: "Go", InputType: "image"}, ReasonSubmits},
		{VerbClick, Subject{Role: "button", Name: "Start over", InputType: "reset"}, ReasonSubmits},
		{VerbClick, Subject{Role: "button", Name: "Next", PaymentForm: true}, ReasonPaymentForm},
		{VerbSelect, Subject{Role: "combobox", Name: "Plan", Option: "Yearly", PaymentForm: true}, ReasonPaymentForm},
		{VerbClick, Subject{Role: "generic", Name: "Next"}, ReasonUnknownRole},
		{VerbClick, Subject{Name: "Next"}, ReasonUnknownRole},
		{VerbFill, Subject{Role: "button", Name: "Name"}, ReasonUnknownRole},
		{VerbFill, Subject{Role: "textbox", Name: "Volume", InputType: "range"}, ReasonUnknownRole},
		{VerbFill, Subject{Role: "textbox", Name: "Agree", InputType: "checkbox"}, ReasonUnknownRole},
		{VerbSelect, Subject{Role: "button", Name: "Country", Option: "Japan"}, ReasonUnknownRole},
		{"press", button("Next"), ReasonUnknownVerb},
		{"Click", button("Next"), ReasonUnknownVerb},
		{"key", button("Next"), ReasonUnknownVerb},
		{"", button("Next"), ReasonUnknownVerb},
		{"done", Subject{}, ReasonUnknownVerb},
	}
	for _, tc := range cases {
		if c, r := Classify(tc.verb, tc.s, ClaimNormal); c != Handoff || r != tc.r {
			t.Errorf("%q %+v = %v %s, want handoff %s", tc.verb, tc.s, c, r, tc.r)
		}
	}
}

func TestNoClassIsAnsweredBySpeech(t *testing.T) {
	for _, c := range []Class{Handoff, Safe, Refused, Class(200)} {
		if SpokenMayAnswer(c) {
			t.Errorf("SpokenMayAnswer(%v) = true", c)
		}
	}
	var zero Class
	if zero != Handoff || zero.String() != "handoff" || Safe.String() != "safe" || Refused.String() != "refused" {
		t.Fatalf("class names: zero %v, %v, %v", zero, Safe, Refused)
	}
}
