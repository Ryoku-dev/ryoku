package feature

import (
	"errors"
	"slices"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// KindApproval is the frame of an approval card on Oni's pill.
const KindApproval = "approval"

// The id prefixes of approval cards: a Hands request a team member Yokai
// hosts raised, and a permission request a hosted harness raised in its
// team task's conversation.
const (
	ApprovalHandsPrefix      = "ta-"
	ApprovalPermissionPrefix = "tp-"
)

// The buttons an approval card may offer, in the order it shows them:
// allow once, allow for the conversation (a permission that is not
// danger-tier only), no, and leaving the answer to the Hyakki app, which
// decides nothing and takes the card off the pill.
const (
	ApprovalOnce   = "once"
	ApprovalAlways = "always"
	ApprovalNo     = "no"
	ApprovalInApp  = "app"
)

// ApprovalOptions are every approval card button, in order.
var ApprovalOptions = []string{ApprovalOnce, ApprovalAlways, ApprovalNo, ApprovalInApp}

// ApprovalTitleMax bounds an approval card's title, in runes.
const ApprovalTitleMax = 400

// ApprovalFrame shows or takes down one approval card beside Oni. Open
// shows it: Title says what is asked, Scope (nested or window) is a Hands
// request's, empty for a permission, and Options are the buttons, each
// answered with oni.consent.answer and the option as the answer. A card
// the pill may not answer (a danger-tier permission, any desktop scope)
// offers only ApprovalInApp. A closed frame carries only its id. It is
// never in a snapshot: the daemon sends the waiting card again to a
// presenter that opens.
type ApprovalFrame struct {
	ID      string   `json:"id"`
	Open    bool     `json:"open"`
	Title   string   `json:"title,omitempty"`
	Scope   string   `json:"scope,omitempty"`
	Options []string `json:"options,omitempty"`
}

func (ApprovalFrame) Kind() string { return KindApproval }

func (f ApprovalFrame) validate() error {
	hands := strings.HasPrefix(f.ID, ApprovalHandsPrefix)
	if !hands && !strings.HasPrefix(f.ID, ApprovalPermissionPrefix) || !approvalSeq(f.ID[min(len(f.ID), 3):]) {
		return errors.New("an approval card's id is ta- or tp- and a number")
	}
	if !f.Open {
		if f.Title != "" || f.Scope != "" || len(f.Options) > 0 {
			return errors.New("a closed approval card carries only its id")
		}
		return nil
	}
	if !protocol.ValidOniText(f.Title, ApprovalTitleMax, false) {
		return errors.New("an approval card's title is one line of at most 400 runes")
	}
	switch {
	case hands && !slices.Contains(protocol.AgentHandsScopes, f.Scope):
		return errors.New("a Hands approval card names the nested desktop or a window")
	case !hands && f.Scope != "":
		return errors.New("a permission card names no scope")
	}
	if len(f.Options) == 0 || f.Options[len(f.Options)-1] != ApprovalInApp {
		return errors.New("an approval card always offers the app")
	}
	last := -1
	for _, option := range f.Options {
		at := slices.Index(ApprovalOptions, option)
		if at <= last {
			return errors.New("approval card options are distinct, known and in order")
		}
		last = at
	}
	if hands && slices.Contains(f.Options, ApprovalAlways) {
		return errors.New("a Hands approval is allowed once only")
	}
	if len(f.Options) > 1 && (!slices.Contains(f.Options, ApprovalOnce) || !slices.Contains(f.Options, ApprovalNo)) {
		return errors.New("an answerable approval card offers allow once and no")
	}
	return nil
}

// approvalSeq is a card's number: 1 to 20 digits, no leading zero.
func approvalSeq(s string) bool {
	if s == "" || len(s) > 20 || s[0] == '0' {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
