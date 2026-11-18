package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHowTo_FlagsMissingTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here's how clone the repository.", NewHowTo(), "Here's how to clone the repository.")
}

func TestHowTo_IgnoresCorrectPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "Here's how to clone the repository.", NewHowTo(), 0)
}

func TestHowTo_FlagsOtherVerbs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Learn how install Rust.", NewHowTo(), "Learn how to install Rust.")
}

func TestHowTo_RosPackageInstall(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can someone explain how install this ROS package on Humble?", NewHowTo(), "Can someone explain how to install this ROS package on Humble?")
}

func TestHowTo_ExtractAndInstallApp(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here’s a quick guide on how install an app you’ve extracted from a tarball.", NewHowTo(), "Here’s a quick guide on how to install an app you’ve extracted from a tarball.")
}

func TestHowTo_DllFiles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This video shows how fix missing DLL files on Windows.", NewHowTo(), "This video shows how to fix missing DLL files on Windows.")
}

func TestHowTo_DofusOnUbuntu(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Full tutorial on how install Dofus under Ubuntu.", NewHowTo(), "Full tutorial on how to install Dofus under Ubuntu.")
}

func TestHowTo_TarGzInstall(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Find out how install software shipped as a .tar.gz archive.", NewHowTo(), "Find out how to install software shipped as a .tar.gz archive.")
}

func TestHowTo_ThriftLibraries(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Anyone know how install the Thrift libraries from source?", NewHowTo(), "Anyone know how to install the Thrift libraries from source?")
}

func TestHowTo_WindowsAdk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lost the Windows ADK again—remind me how install it?", NewHowTo(), "Lost the Windows ADK again—remind me how to install it?")
}

func TestHowTo_AccountingErrors(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Eight common accounting errors and how fix them.", NewHowTo(), "Eight common accounting errors and how to fix them.")
}

func TestHowTo_SentenceFragments(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here’s what sentence fragments are and how fix them.", NewHowTo(), "Here’s what sentence fragments are and how to fix them.")
}

func TestHowTo_ZipperSlider(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Quick demo on how fix a broken zipper slider.", NewHowTo(), "Quick demo on how to fix a broken zipper slider.")
}

func TestHowTo_DoorLock(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tips on how fix a door that won’t lock.", NewHowTo(), "Tips on how to fix a door that won’t lock.")
}

func TestHowTo_AlreadyCorrectInstall(t *testing.T) {
	linttest.AssertLintCount(t, "See how to install the package with apt.", NewHowTo(), 0)
}

func TestHowTo_AlreadyCorrectFix(t *testing.T) {
	linttest.AssertLintCount(t, "He showed me how to fix the zipper in ten minutes.", NewHowTo(), 0)
}

func TestHowTo_HowAreYou(t *testing.T) {
	linttest.AssertLintCount(t, "How are you?", NewHowTo(), 0)
}

func TestHowTo_HowCalmYouAre(t *testing.T) {
	linttest.AssertLintCount(t, "I like how calm you are.", NewHowTo(), 0)
}

func TestHowTo_HowWillYouMakeUp(t *testing.T) {
	linttest.AssertLintCount(t, "How will you make up for your mistakes?", NewHowTo(), 0)
}

func TestHowTo_StorytellingClause(t *testing.T) {
	linttest.AssertLintCount(t, "I will tell about how leaving my husband led to my dog winning a Nobel Prize.", NewHowTo(), 0)
}

func TestHowTo_DontFlagHowDidYou(t *testing.T) {
	linttest.AssertLintCount(t, "How did you get to school every day?", NewHowTo(), 0)
}

func TestHowTo_DontFlagHowCome(t *testing.T) {
	linttest.AssertLintCount(t, "How come this has to be a special case?", NewHowTo(), 0)
}

func TestHowTo_AllowsHowHas(t *testing.T) {
	linttest.AssertLintCount(t, "How Has This Been Tested?", NewHowTo(), 0)
}

func TestHowTo_Issue1492(t *testing.T) {
	linttest.AssertNoLints(t, "I hope to provide some insight into correct HTML formatting, in addition to how authors can avoid these issues.", NewHowTo())
	linttest.AssertNoLints(t, "But how does something like this...", NewHowTo())
}

func TestHowTo_AllowIssue1298(t *testing.T) {
	linttest.AssertNoLints(t, "The story of how and why things came to this point.", NewHowTo())
}

func TestHowTo_DontFlagFalsePositivePr1846(t *testing.T) {
	linttest.AssertNoLints(t, "About how Microsoft, Google, and others are training people in Rust.", NewHowTo())
}

func TestHowTo_DontFlagFalsePositives1492HowIndexes(t *testing.T) {
	linttest.AssertNoLints(t, "controls how indexes will be added to unwrapped keys of flat array-like objects", NewHowTo())
}

func TestHowTo_DontFlagHowProperNounHandles(t *testing.T) {
	linttest.AssertNoLints(t, "rewrites how Wine handles file locking", NewHowTo())
}

func TestHowTo_DontFlagHowWorkIsStructured(t *testing.T) {
	linttest.AssertNoLints(t, "They need to rethink how work is structured and valued.", NewHowTo())
}

func TestHowTo_DontFlagHowFormSubmissionsAreDelivered(t *testing.T) {
	linttest.AssertNoLints(t, "The Mail tab dictates how form submissions are delivered to you.", NewHowTo())
}

func TestHowTo_DontFlagHowAccessToFoodShapedRevolutions(t *testing.T) {
	linttest.AssertNoLints(t, "We analyze how access to food shaped political ideologies.", NewHowTo())
}

func TestHowTo_Issue2124(t *testing.T) {
	linttest.AssertNoLints(t, "I like how discord shows Spotify status on your profile.", NewHowTo())
	linttest.AssertNoLints(t, "To be determined based on how error handling is done in new paradigm.", NewHowTo())
}
