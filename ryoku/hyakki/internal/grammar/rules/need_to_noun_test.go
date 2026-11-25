package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNeedToNoun_FlagsNeedToNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I need to information now.", NewNeedToNoun(), "I need the information now.")
}

func TestNeedToNoun_AllowsNeedToVerb(t *testing.T) {
	linttest.AssertLintCount(t, "I need to leave now.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToFinish(t *testing.T) {
	linttest.AssertLintCount(t, "I need to finish this report by tomorrow.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToCall(t *testing.T) {
	linttest.AssertLintCount(t, "You need to call your mother tonight.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToTalk(t *testing.T) {
	linttest.AssertLintCount(t, "We need to talk about the budget.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToLeave(t *testing.T) {
	linttest.AssertLintCount(t, "They need to leave early to catch the train.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToPractice(t *testing.T) {
	linttest.AssertLintCount(t, "She needs to practice her German more often.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToFix(t *testing.T) {
	linttest.AssertLintCount(t, "He needs to fix his bike before the weekend.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToDecide(t *testing.T) {
	linttest.AssertLintCount(t, "We need to decide where to go for dinner.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToUpdate(t *testing.T) {
	linttest.AssertLintCount(t, "You need to update your password.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToTake(t *testing.T) {
	linttest.AssertLintCount(t, "I need to take a break and get some fresh air.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeedToClean(t *testing.T) {
	linttest.AssertLintCount(t, "They need to clean the house before guests arrive.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AvoidsFalsePositiveForNeedToVerify(t *testing.T) {
	linttest.AssertLintCount(t, "I need to verify the expenses before submission.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_FlagsNeedToCompiler(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We simply don't need to compiler to do as much work anymore.", NewNeedToNoun(), "We simply don't need the compiler to do as much work anymore.")
}

func TestNeedToNoun_FlagsNeedToVerification(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I need to verification before logging in.", NewNeedToNoun(), "I need the verification before logging in.")
}

func TestNeedToNoun_AllowsNeedToReport(t *testing.T) {
	linttest.AssertNoLints(t, "We need to report before the meeting starts.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToPassword(t *testing.T) {
	linttest.AssertNoLints(t, "You need to password to access the server.", NewNeedToNoun())
}

func TestNeedToNoun_FlagsNeedToData(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They need to data analyzed by tomorrow.", NewNeedToNoun(), "They need the data analyzed by tomorrow.")
}

func TestNeedToNoun_FlagsNeedToApproval(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She will need to approval of her manager first.", NewNeedToNoun(), "She will need the approval of her manager first.")
}

func TestNeedToNoun_AllowsNeedToBackup(t *testing.T) {
	linttest.AssertNoLints(t, "We might need to backup if the main system fails.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToPermit(t *testing.T) {
	linttest.AssertNoLints(t, "He didn’t realize he would need to permit to film there.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToTools(t *testing.T) {
	linttest.AssertNoLints(t, "You’ll need to right tools to fix that.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToContext(t *testing.T) {
	linttest.AssertNoLints(t, "We need to context to make sense of his decision.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToFunds(t *testing.T) {
	linttest.AssertNoLints(t, "They need to funds released before construction begins.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToSilence(t *testing.T) {
	linttest.AssertNoLints(t, "I need to silence to think clearly.", NewNeedToNoun())
}

func TestNeedToNoun_FlagsNeedsToApproval(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She needs to approval from her advisor.", NewNeedToNoun(), "She needs the approval from her advisor.")
}

func TestNeedToNoun_AvoidsFalsePositiveForNeedsToCoordinate(t *testing.T) {
	linttest.AssertLintCount(t, "She needs to collaborate with everyone on the plan.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_FlagsNeedsToVerification(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He needs to verification ready before the audit.", NewNeedToNoun(), "He needs the verification ready before the audit.")
}

func TestNeedToNoun_AllowsNeedsToFinalize(t *testing.T) {
	linttest.AssertLintCount(t, "She needs to finalize the schedule.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsNeededToPermit(t *testing.T) {
	linttest.AssertNoLints(t, "They needed to permit before entering the site.", NewNeedToNoun())
}

func TestNeedToNoun_AvoidsFalsePositiveForNeededToExplain(t *testing.T) {
	linttest.AssertLintCount(t, "They needed to explain the new policy carefully.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_CatchesFalseNegativeForNeededToAuthorization(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They needed to authorization before proceeding.", NewNeedToNoun(), "They needed the authorization before proceeding.")
}

func TestNeedToNoun_AllowsNeededToFile(t *testing.T) {
	linttest.AssertLintCount(t, "They needed to file the paperwork before noon.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_FlagsNeedingToDocumentation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Needing to documentation slowed the entire process.", NewNeedToNoun(), "Needing the documentation slowed the entire process.")
}

func TestNeedToNoun_AvoidsFalsePositiveForNeedingToCalibrate(t *testing.T) {
	linttest.AssertLintCount(t, "Needing to calibrate the equipment delayed us slightly.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_CatchesFalseNegativeForNeedingToConfirmation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Needing to confirmation from legal stalled the launch.", NewNeedToNoun(), "Needing the confirmation from legal stalled the launch.")
}

func TestNeedToNoun_AllowsNeedingToCall(t *testing.T) {
	linttest.AssertLintCount(t, "Needing to call your mother is stressful.", NewNeedToNoun(), 0)
}

func TestNeedToNoun_AllowsIssue2252(t *testing.T) {
	linttest.AssertNoLints(t, "Things I need to do today:", NewNeedToNoun())
}

func TestNeedToNoun_AllowsInstall(t *testing.T) {
	linttest.AssertNoLints(t, "You need to install it separately, as it's a standalone application.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsLay(t *testing.T) {
	linttest.AssertNoLints(t, "Okay, this is a long one, but I feel like I need to lay everything out.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsOvercome(t *testing.T) {
	linttest.AssertNoLints(t, "We believe every family deserves the opportunity to flourish, and we are committed to providing the resources they need to overcome adversity.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToRunInto2433(t *testing.T) {
	linttest.AssertNoLints(t, "So that they don't need to run into this problem in the future.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToMatch2446(t *testing.T) {
	linttest.AssertNoLints(t, "You don't need to match string errors explicitly.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToMatchExactly2446(t *testing.T) {
	linttest.AssertNoLints(t, "They need to match exactly.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToUsePhpCodeFuzz(t *testing.T) {
	linttest.AssertNoLints(t, "To display the custom field data on your website, you'll likely need to use PHP code within your theme files.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToDisplayImagesFuzz(t *testing.T) {
	linttest.AssertNoLints(t, "I'm building a photography portfolio site for a client and need to display images in a responsive gallery.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToBuildBrighterFuturesFuzz(t *testing.T) {
	linttest.AssertNoLints(t, "At Haven House, our mission is to provide families with the resources they need to build brighter futures.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToRedefineSuccessFuzz(t *testing.T) {
	linttest.AssertNoLints(t, "We need to redefine success to include wellbeing and sustainability.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToShiftFromFuzz(t *testing.T) {
	linttest.AssertNoLints(t, "We need to shift from a deficit model to an abundance model.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToResearchAndChooseFuzz(t *testing.T) {
	linttest.AssertNoLints(t, "This means you need to research and choose adapters carefully.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToModelHealthyHabitsFuzz(t *testing.T) {
	linttest.AssertNoLints(t, "Leaders need to model healthy work habits and create a safe space for employees.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToStart2320(t *testing.T) {
	linttest.AssertNoLints(t, "You need to start the server before running tests.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToHave2320(t *testing.T) {
	linttest.AssertNoLints(t, "You need to have a valid license to use this software.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToConfigure2320(t *testing.T) {
	linttest.AssertNoLints(t, "You need to configure the database connection first.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToSet2320(t *testing.T) {
	linttest.AssertNoLints(t, "You need to set the environment variable before deploying.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToSend2320(t *testing.T) {
	linttest.AssertNoLints(t, "You need to send the request with the correct headers.", NewNeedToNoun())
}

func TestNeedToNoun_AllowsNeedToReceive2320(t *testing.T) {
	linttest.AssertNoLints(t, "You need to receive confirmation before proceeding.", NewNeedToNoun())
}
