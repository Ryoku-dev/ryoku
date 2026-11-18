package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestChangeTack_ChangeTactAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "change tact", NewChangeTack(), "change tack")
}

func TestChangeTack_ChangedTacksAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "changed tacks", NewChangeTack(), "changed tack")
}

func TestChangeTack_ChangesTactsAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "changes tacts", NewChangeTack(), "changes tack")
}

func TestChangeTack_ChangingTactAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "changing tact", NewChangeTack(), "changing tack")
}

func TestChangeTack_ChangeOfTacksAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "change of tacks", NewChangeTack(), "change of tack")
}

func TestChangeTack_ChangeOfTactRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Change of tact : come give your concerns - Death Knight", NewChangeTack(), "Change of tack : come give your concerns - Death Knight")
}

func TestChangeTack_ChangeOfTactsRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "2013.08.15 - A Change of Tacts | Hero MUX Wiki | Fandom", NewChangeTack(), "2013.08.15 - A Change of Tack | Hero MUX Wiki | Fandom")
}

func TestChangeTack_ChangingOfTacksRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Duffy's changing of tacks hidden in her poetry collection ...", NewChangeTack(), "Duffy's changing of tack hidden in her poetry collection ...")
}

func TestChangeTack_ChangesOfTactRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "While the notes and the changes of tact started to ...", NewChangeTack(), "While the notes and the changes of tack started to ...")
}

func TestChangeTack_ChangedMyTact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have changed my tact this year, and have two second dates in the next week.", NewChangeTack(), "I have changed my tack this year, and have two second dates in the next week.")
}

func TestChangeTack_ChangedOurTact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That being said we have changed our tact slightly and gone for making all UI elements lazy.", NewChangeTack(), "That being said we have changed our tack slightly and gone for making all UI elements lazy.")
}

func TestChangeTack_ChangeYourTact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you've ever heard the phrase “you've got to change your tact”, this is probably where it comes from.", NewChangeTack(), "If you've ever heard the phrase “you've got to change your tack”, this is probably where it comes from.")
}

func TestChangeTack_ChangeHisTact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why did Sephiroth change his tact with Cloud midway through the game?", NewChangeTack(), "Why did Sephiroth change his tack with Cloud midway through the game?")
}

func TestChangeTack_ChangedHerTact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Only the last commitment ceremony I think she changed her tact and went on about George needing to be the real George.", NewChangeTack(), "Only the last commitment ceremony I think she changed her tack and went on about George needing to be the real George.")
}

func TestChangeTack_ChangeItsTact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The show seems to change its tact depending on the episode.", NewChangeTack(), "The show seems to change its tack depending on the episode.")
}

func TestChangeTack_ChangingItsTactApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "FYI, USL is changing it's tact internally about MLS II teams.", NewChangeTack(), "FYI, USL is changing it's tack internally about MLS II teams.")
}

func TestChangeTack_ChangesTheirTact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As we become inoculated to attention grifts, the grifter changes their tact.", NewChangeTack(), "As we become inoculated to attention grifts, the grifter changes their tack.")
}

func TestChangeTack_DifferentTact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So, I recently took a different tact: I put all my models etc. in a single folder.", NewChangeTack(), "So, I recently took a different tack: I put all my models etc. in a single folder.")
}
