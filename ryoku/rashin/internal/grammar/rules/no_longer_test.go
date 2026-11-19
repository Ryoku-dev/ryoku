package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNoLonger_IgnoreThan(t *testing.T) {
	linttest.AssertNoLints(t, "My arm is not longer than my leg.", NewNoLonger())
}

func TestNoLonger_FixCanModal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and I've found out that I not longer can launch Kitty from the menus", NewNoLonger(), "and I've found out that I no longer can launch Kitty from the menus")
}

func TestNoLonger_FixDonePastParticiple(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've noticed that the ML stuff is not longer done on the more recent photos.", NewNoLonger(), "I've noticed that the ML stuff is no longer done on the more recent photos.")
}

func TestNoLonger_FixExist(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Vendoring means that the transitive dependencies do not longer exist from the point of view of the consumer.", NewNoLonger(), "Vendoring means that the transitive dependencies do no longer exist from the point of view of the consumer.")
}

func TestNoLonger_FixExists3rdPersonSingularPresent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this script is mentioned in the RF3 Readme but the script not longer exists", NewNoLonger(), "this script is mentioned in the RF3 Readme but the script no longer exists")
}

func TestNoLonger_FixRender(t *testing.T) {
	linttest.AssertSuggestionResult(t, "auto comments will not longer render annotations in such a way as to make them valid annotation links", NewNoLonger(), "auto comments will no longer render annotations in such a way as to make them valid annotation links")
}

func TestNoLonger_FixSavedRegularPast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "edit notes are not longer saved on mobile", NewNoLonger(), "edit notes are no longer saved on mobile")
}

func TestNoLonger_FixSavingPresentParticiple(t *testing.T) {
	linttest.AssertSuggestionResult(t, "After Updating to 4.3.2 from 4.2.1 the JSON Editor is not longer saving the metadata.", NewNoLonger(), "After Updating to 4.3.2 from 4.2.1 the JSON Editor is no longer saving the metadata.")
}

func TestNoLonger_FixWrittenPastParticiple(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I get this error and the fasta file is not longer written", NewNoLonger(), "I get this error and the fasta file is no longer written")
}

func TestNoLonger_FixAble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am not longer able to set multi-cursors in Zed 0.190.6.", NewNoLonger(), "I am no longer able to set multi-cursors in Zed 0.190.6.")
}

func TestNoLonger_FixAffordable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No not Oakland, it's not longer affordable.", NewNoLonger(), "No not Oakland, it's no longer affordable.")
}

func TestNoLonger_FixBad(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How many times does this have to happen before its not longer bad luck?", NewNoLonger(), "How many times does this have to happen before its no longer bad luck?")
}

func TestNoLonger_FixBest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "AWS Java V1 is not longer best practice as specified in this Github page", NewNoLonger(), "AWS Java V1 is no longer best practice as specified in this Github page")
}

func TestNoLonger_FixEffective(t *testing.T) {
	linttest.AssertSuggestionResult(t, "when you delete those keys from the dict, it is not longer effective", NewNoLonger(), "when you delete those keys from the dict, it is no longer effective")
}

func TestNoLonger_FixEmpty(t *testing.T) {
	linttest.AssertSuggestionResult(t, "not only set as username, it sets common name as well and is not longer empty", NewNoLonger(), "not only set as username, it sets common name as well and is no longer empty")
}

func TestNoLonger_FixEnough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the message body is not longer enough", NewNoLonger(), "the message body is no longer enough")
}

func TestNoLonger_FixEqual(t *testing.T) {
	linttest.AssertSuggestionResult(t, "once the size of the current batch is not longer equal to batch_size , I used the temporary batch", NewNoLonger(), "once the size of the current batch is no longer equal to batch_size , I used the temporary batch")
}

func TestNoLonger_FixEquivalent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the lambda is not longer equivalent to how std::isspace would behave as a unary predicate", NewNoLonger(), "the lambda is no longer equivalent to how std::isspace would behave as a unary predicate")
}

func TestNoLonger_FixFree(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so if i understand it correct, myteslamate is not longer free? ", NewNoLonger(), "so if i understand it correct, myteslamate is no longer free? ")
}

func TestNoLonger_FixGood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just in case that link is not longer good I'll reproduce the code here.", NewNoLonger(), "Just in case that link is no longer good I'll reproduce the code here.")
}

func TestNoLonger_FixNear(t *testing.T) {
	linttest.AssertSuggestionResult(t, "reminder that they are not longer near each other", NewNoLonger(), "reminder that they are no longer near each other")
}

func TestNoLonger_FixOpen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "removing old breakpoints from a project which was not longer open", NewNoLonger(), "removing old breakpoints from a project which was no longer open")
}

func TestNoLonger_FixPossible(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As far as I can set tell it is not longer possible to set these programmatically.", NewNoLonger(), "As far as I can set tell it is no longer possible to set these programmatically.")
}

func TestNoLonger_FixRelevant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "individual remuneration is not longer relevant as we can produce enough", NewNoLonger(), "individual remuneration is no longer relevant as we can produce enough")
}

func TestNoLonger_FixSufficient(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the fichier.close() command is not longer sufficient to close the file", NewNoLonger(), "the fichier.close() command is no longer sufficient to close the file")
}
