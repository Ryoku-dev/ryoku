package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPhrasalVerbAsCompoundNoun_FlagBreakupAndWorkout(t *testing.T) {
	linttest.AssertLintCount(t, "I will never breakup with Gym. We just seem to workout.", NewPhrasalVerbAsCompoundNoun(), 2)
}

func TestPhrasalVerbAsCompoundNoun_CorrectBreakupAndWorkout(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I will never breakup with Gym. We just seem to workout.", NewPhrasalVerbAsCompoundNoun(), "I will never break up with Gym. We just seem to work out.")
}

func TestPhrasalVerbAsCompoundNoun_FlagRollbackUsedAsAVerbIssue3239(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I already did a rollback. I'm not going to rollback again.", NewPhrasalVerbAsCompoundNoun(), "I already did a rollback. I'm not going to roll back again.")
}

func TestPhrasalVerbAsCompoundNoun_FlagComebackUsedAsAVerbIssue3239(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I already did a comeback. I'm not going to comeback again.", NewPhrasalVerbAsCompoundNoun(), "I already did a comeback. I'm not going to come back again.")
}

func TestPhrasalVerbAsCompoundNoun_DontFlagRollbackOrComebackAsNounsIssue3239(t *testing.T) {
	linttest.AssertNoLints(t, "I already did a rollback. It was quite a comeback.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagRandomWordsThatHappenToEndLikeAParticle(t *testing.T) {
	linttest.AssertNoLints(t, "I like bacon.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagNonVerbParticles(t *testing.T) {
	linttest.AssertNoLints(t, "non", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_CorrectAfterI(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I backup", NewPhrasalVerbAsCompoundNoun(), "I back up")
}

func TestPhrasalVerbAsCompoundNoun_CorrectAfterWe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we breakup", NewPhrasalVerbAsCompoundNoun(), "we break up")
}

func TestPhrasalVerbAsCompoundNoun_DontFlagCheckin(t *testing.T) {
	linttest.AssertNoLints(t, "checkin", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagCleanup(t *testing.T) {
	linttest.AssertNoLints(t, "cleanup", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_CorrectAfterYouLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you checkout", NewPhrasalVerbAsCompoundNoun(), "you check out")
}

func TestPhrasalVerbAsCompoundNoun_CorrectAfterYouCapitalized(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You checkout", NewPhrasalVerbAsCompoundNoun(), "You check out")
}

func TestPhrasalVerbAsCompoundNoun_FlagCheckoutAfterYou(t *testing.T) {
	linttest.AssertLintCount(t, "you checkout", NewPhrasalVerbAsCompoundNoun(), 1)
}

func TestPhrasalVerbAsCompoundNoun_CorrectAfterTheyLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "they cleanup", NewPhrasalVerbAsCompoundNoun(), "they clean up")
}

func TestPhrasalVerbAsCompoundNoun_FlagCleanupAfterThey(t *testing.T) {
	linttest.AssertLintCount(t, "they cleanup", NewPhrasalVerbAsCompoundNoun(), 1)
}

func TestPhrasalVerbAsCompoundNoun_DontFlagDictionaryLookup(t *testing.T) {
	linttest.AssertNoLints(t, "dictionary lookup", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_FlagCouplesBreakup(t *testing.T) {
	linttest.AssertLintCount(t, "couples breakup", NewPhrasalVerbAsCompoundNoun(), 1)
}

func TestPhrasalVerbAsCompoundNoun_DontFlagGallon(t *testing.T) {
	linttest.AssertNoLints(t, "gallon", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagStartupFunding(t *testing.T) {
	linttest.AssertNoLints(t, "Yarvin has actually given startup funding. They hang out and party together", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagHugeMarkup(t *testing.T) {
	linttest.AssertNoLints(t, "Sell it back to Russia at a huge markup.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagAnotherLayoff(t *testing.T) {
	linttest.AssertNoLints(t, "And now just announced another layoff", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagAThreatOrShakedown(t *testing.T) {
	t.Skip("\"Shakedown\" is a compound noun -- it's part of a comma-separated list with another noun \"threat\"\nBut this is not easy to check for so is not implemented yet.")
	linttest.AssertNoLints(t, "Just a threat or Shakedown.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagAFlyover(t *testing.T) {
	linttest.AssertNoLints(t, "if I'm the Brits I'm doing a flyover", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagMafiaStyleShakedown(t *testing.T) {
	linttest.AssertNoLints(t, "Basically it's kind of a mafia style shakedown of Ukraine", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagMyMeetupRepository(t *testing.T) {
	linttest.AssertNoLints(t, "I might have in my Meetup repository", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_IgnoreMultiWord(t *testing.T) {
	linttest.AssertNoLints(t, "I like this add-on!", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagListOfNouns1298(t *testing.T) {
	linttest.AssertNoLints(t, "A printable format and layout.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagOovNvimPlugin1280(t *testing.T) {
	linttest.AssertNoLints(t, "This is the nvim plugin for you.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_FlagTitleCase(t *testing.T) {
	linttest.AssertLintCount(t, "I Will Never Breakup With Gym. We Just Seem To Workout.", NewPhrasalVerbAsCompoundNoun(), 2)
}

func TestPhrasalVerbAsCompoundNoun_FlagAllCaps(t *testing.T) {
	linttest.AssertLintCount(t, "I WILL NEVER BREAKUP WITH GYM. WE JUST SEEM TO WORKOUT.", NewPhrasalVerbAsCompoundNoun(), 2)
}

func TestPhrasalVerbAsCompoundNoun_FalsePositiveIssue1495(t *testing.T) {
	linttest.AssertNoLints(t, "Color schemes are available by using the Style Settings plugin.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagThanksALotLinterDescription(t *testing.T) {
	linttest.AssertLintCount(t, "Thanks a lot` is the fixed, widely accepted form, while variants like `thanks lot` or `thanks alot` are non-standard and can jar readers.", NewPhrasalVerbAsCompoundNoun(), 0)
}

func TestPhrasalVerbAsCompoundNoun_DontFlagBackupLocation(t *testing.T) {
	linttest.AssertNoLints(t, "Backup location: `%APPDATA%\\Cursor\\User\\globalStorage\\backups`", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagBackupPlan(t *testing.T) {
	linttest.AssertNoLints(t, "Every backup plan is unique, based on your risk assessment.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagBackupProgram(t *testing.T) {
	linttest.AssertNoLints(t, "restic is a backup program that is fast, efficient and secure", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagBackupSolutionOrBackupProblems(t *testing.T) {
	linttest.AssertNoLints(t, "NPBackup is a multiparadigm backup solution which tries to solve two major backup problems", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagBackupUtilitiesBackupSystemOrBackupSnapshots(t *testing.T) {
	linttest.AssertNoLints(t, "GitHub Enterprise Server Backup Utilities is a backup system you install on a separate host, which takes backup snapshots", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagBackupImages(t *testing.T) {
	linttest.AssertNoLints(t, "This App creates and stores backup images of your Nextcloud.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_FixBackupIndividualApps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It requires root and allows you to backup individual apps and their data.", NewPhrasalVerbAsCompoundNoun(), "It requires root and allows you to back up individual apps and their data.")
}

func TestPhrasalVerbAsCompoundNoun_DontFlagBackupStrategy(t *testing.T) {
	linttest.AssertNoLints(t, "This is for you if you want to quickly set up a backup strategy without much fuss.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagHelmBackupPlugin(t *testing.T) {
	linttest.AssertNoLints(t, "Helm Backup Plugin.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagCallbackFunction(t *testing.T) {
	linttest.AssertNoLints(t, "By the time the `setTimeout` callback function was invoked", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPlaybackLatency(t *testing.T) {
	linttest.AssertNoLints(t, "Low-Latency HLS is a recently standardized variant of the protocol that allows to greatly reduce playback latency.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagWorkoutConstraints(t *testing.T) {
	linttest.AssertNoLints(t, "Workout constraints", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagWorkoutPreference(t *testing.T) {
	linttest.AssertNoLints(t, "Workout preference", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagRolloutStatus(t *testing.T) {
	linttest.AssertNoLints(t, "Rollout Status of Latest Image Release", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_FontFlagWithPlugin(t *testing.T) {
	linttest.AssertNoLints(t, "**Xcode** (8.0+, otherwise [with plugin](https://github.com/robertvojta/LigatureXcodePlugin))", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagAndLayoutOfData(t *testing.T) {
	linttest.AssertNoLints(t, "shape, memory space, and layout of data, while performing the complicated indexing for the user", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagInNounListWithoutSpaceAfterComma(t *testing.T) {
	linttest.AssertNoLints(t, "shape, memory space,and layout of data", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagLayoutEstimation(t *testing.T) {
	linttest.AssertNoLints(t, "Layout estimation focuses on predicting architectural elements, i.e., walls, doors, and windows, within an indoor scene.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPluginThat(t *testing.T) {
	linttest.AssertNoLints(t, "plugin that provides way for auto-loading of Golang SDK", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagLoadBalancingAndFailover(t *testing.T) {
	linttest.AssertNoLints(t, "resilient mid-tier load balancing and failover", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPluginFor(t *testing.T) {
	linttest.AssertNoLints(t, "Plugin for text editors and IDEs.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagMarkupLanguage(t *testing.T) {
	linttest.AssertNoLints(t, "Markup language used for websites & web apps.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPluginEcosystemOrPluginDevelopment(t *testing.T) {
	linttest.AssertNoLints(t, "## 🧩 Plugin Ecosystem\n### Plugin Development", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPluginFilesOrPluginPackages(t *testing.T) {
	linttest.AssertNoLints(t, "plugin files between plugin packages installed with pip must have unique filenames.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPluginDocs(t *testing.T) {
	linttest.AssertNoLints(t, "building your own plugin: [Plugin Docs]", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPluginSuite(t *testing.T) {
	linttest.AssertNoLints(t, "An all-in-one digital audio workstation (DAW) and plugin suite.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagHackerNewsThrowbackMachine(t *testing.T) {
	linttest.AssertNoLints(t, "| Hacker News Throwback Machine | Shows what was popular on Hacker News on this day in previous years.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPluginInterface(t *testing.T) {
	linttest.AssertNoLints(t, "[Plugin interface]", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_Issue1918(t *testing.T) {
	linttest.AssertNoLints(t, "Boost your productivity with our JetBrains plugin!", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagPopUp2217(t *testing.T) {
	linttest.AssertNoLints(t, "Popup window instead of command line.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_Issue1772(t *testing.T) {
	linttest.AssertNoLints(t, "By default, only one tile size is instantiated for each data type, math instruction, and layout.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_Issue2369(t *testing.T) {
	linttest.AssertNoLints(t, "## Plugin developer documentation", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_Issue2505DontFlagBackupLinks(t *testing.T) {
	linttest.AssertNoLints(t, "seamless switching to one of the backup links ideally happens without packet drops", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_Issue2505CorrectSetupButDontFlagBackupLink(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How to properly setup a backup link (and have it act like a backup again after stop/start of master link)", NewPhrasalVerbAsCompoundNoun(), "How to properly set up a backup link (and have it act like a backup again after stop/start of master link)")
}

func TestPhrasalVerbAsCompoundNoun_DontFlagMeltdown3591(t *testing.T) {
	linttest.AssertNoLints(t, "Unfortunately, Meltdown ended up being problematic.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagSetupBeforeModalIssue4627(t *testing.T) {
	linttest.AssertNoLints(t, "setup can optionally install Hopper with approval", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_DontFlagCompoundNounBeforeOtherModals(t *testing.T) {
	linttest.AssertNoLints(t, "Cleanup will remove the cache.", NewPhrasalVerbAsCompoundNoun())
	linttest.AssertNoLints(t, "Backup should run nightly.", NewPhrasalVerbAsCompoundNoun())
}

func TestPhrasalVerbAsCompoundNoun_FlagPhrasalVerbEndingAClauseBeforeAModal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "People that don't cleanup can and will be banned from the space.", NewPhrasalVerbAsCompoundNoun(), "People that don't clean up can and will be banned from the space.")
	linttest.AssertSuggestionResult(t, "Couples that decide to breakup can still choose to remain good friends.", NewPhrasalVerbAsCompoundNoun(), "Couples that decide to break up can still choose to remain good friends.")
}
