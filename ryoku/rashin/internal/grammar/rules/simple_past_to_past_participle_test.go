package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSimplePastToPastParticiple_CorrectHaveWent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have went into the btle.py file and added a print statement in _connect()", NewSimplePastToPastParticiple(), "I have gone into the btle.py file and added a print statement in _connect()")
}

func TestSimplePastToPastParticiple_CorrectHadWent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not sure if TroLoos had went from Tasmota->minimal->Tasmota, or directly Minimal->Tasmota, but going ESPHome->Minimal->Tasmota is not possible", NewSimplePastToPastParticiple(), "Not sure if TroLoos had gone from Tasmota->minimal->Tasmota, or directly Minimal->Tasmota, but going ESPHome->Minimal->Tasmota is not possible")
}

func TestSimplePastToPastParticiple_CorrectHavingWent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Having went through the setup guidelines and picking react starter, running npm run watch results in an error", NewSimplePastToPastParticiple(), "Having gone through the setup guidelines and picking react starter, running npm run watch results in an error")
}

func TestSimplePastToPastParticiple_CorrectHasWent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would like to report that the package request which you are loading has went into maintenance mode.", NewSimplePastToPastParticiple(), "I would like to report that the package request which you are loading has gone into maintenance mode.")
}

func TestSimplePastToPastParticiple_CorrectHaveWrote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and while people have wrote partial ImGuiStyle save and ...", NewSimplePastToPastParticiple(), "and while people have written partial ImGuiStyle save and ...")
}

func TestSimplePastToPastParticiple_CorrectHasCame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and mail has came to a work account", NewSimplePastToPastParticiple(), "and mail has come to a work account")
}

func TestSimplePastToPastParticiple_CorrectHaveTook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Keychain took longer than I'd like it to have took, but it still works", NewSimplePastToPastParticiple(), "The Keychain took longer than I'd like it to have taken, but it still works")
}

func TestSimplePastToPastParticiple_CorrectHaveDid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so I have did like below: cd ~/.pub-cache/hosted/pub.dev/", NewSimplePastToPastParticiple(), "so I have done like below: cd ~/.pub-cache/hosted/pub.dev/")
}

func TestSimplePastToPastParticiple_CorrectHasFell(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ScopedHistory instance has fell out of scope ...", NewSimplePastToPastParticiple(), "ScopedHistory instance has fallen out of scope ...")
}

func TestSimplePastToPastParticiple_CorrectHaveBroke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "PlanningEnitity to see the hard constraints that it may have broke", NewSimplePastToPastParticiple(), "PlanningEnitity to see the hard constraints that it may have broken")
}

func TestSimplePastToPastParticiple_CorrectHadBegan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I had began learning Android App development since Aug 2021", NewSimplePastToPastParticiple(), "I had begun learning Android App development since Aug 2021")
}

func TestSimplePastToPastParticiple_CorrectHaveGave(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm not aware we have gave up SM75, why are you asking this?", NewSimplePastToPastParticiple(), "I'm not aware we have given up SM75, why are you asking this?")
}

func TestSimplePastToPastParticiple_CorrectHaveSaw(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have saw that your paper has been accepted by JAIR", NewSimplePastToPastParticiple(), "I have seen that your paper has been accepted by JAIR")
}

func TestSimplePastToPastParticiple_CorrectHaveSpoke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so i may have spoke in error", NewSimplePastToPastParticiple(), "so i may have spoken in error")
}

func TestSimplePastToPastParticiple_CorrectHasBecame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but it has became failed after v2.6.1", NewSimplePastToPastParticiple(), "but it has become failed after v2.6.1")
}

func TestSimplePastToPastParticiple_CorrectHaveKnew(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Oh, I have knew this. You can decrypted it in \"Assetstudio\".", NewSimplePastToPastParticiple(), "Oh, I have known this. You can decrypted it in \"Assetstudio\".")
}

func TestSimplePastToPastParticiple_CorrectHaveDrank(t *testing.T) {
	linttest.AssertSuggestionResult(t, "User should be able to see approximately how much water they have drank today", NewSimplePastToPastParticiple(), "User should be able to see approximately how much water they have drunk today")
}

func TestSimplePastToPastParticiple_BeingWoke(t *testing.T) {
	t.Skip("'Woke' is also an adjective these days")
	linttest.AssertSuggestionResult(t, "and the containers will not being woke up until I execute a \"docker ps\"", NewSimplePastToPastParticiple(), "and the containers will not being woken up until I execute a \"docker ps\"")
}

func TestSimplePastToPastParticiple_CorrectHasFlew(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Well time has flew and I was quite busy but I remember this conversation so I am sharing this with you.", NewSimplePastToPastParticiple(), "Well time has flown and I was quite busy but I remember this conversation so I am sharing this with you.")
}

func TestSimplePastToPastParticiple_CorrectBeingStole(t *testing.T) {
	linttest.AssertSuggestionResult(t, "any requests to obtain the hostname will return the hostname of the container being stole", NewSimplePastToPastParticiple(), "any requests to obtain the hostname will return the hostname of the container being stolen")
}

func TestSimplePastToPastParticiple_CorrectAreBroke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They all worked wonderfully under 3.4.2 and all are broke under 3.5.1.", NewSimplePastToPastParticiple(), "They all worked wonderfully under 3.4.2 and all are broken under 3.5.1.")
}

func TestSimplePastToPastParticiple_CorrectWereGave(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Some devices were gave up during a storm recently, but some are still the same as before.", NewSimplePastToPastParticiple(), "Some devices were given up during a storm recently, but some are still the same as before.")
}

func TestSimplePastToPastParticiple_CorrectBeSaw(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Currently, it's 14560/14550 for default mavlink RX/TX, which can be saw in wfb-cli .", NewSimplePastToPastParticiple(), "Currently, it's 14560/14550 for default mavlink RX/TX, which can be seen in wfb-cli .")
}

func TestSimplePastToPastParticiple_CorrectWasBegan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The initial intent, when v1alpha3 was began, was that almost all usages of InitConfiguration outside of kubeadm init code, could be easily replaced", NewSimplePastToPastParticiple(), "The initial intent, when v1alpha3 was begun, was that almost all usages of InitConfiguration outside of kubeadm init code, could be easily replaced")
}

func TestSimplePastToPastParticiple_CorrectWasGave(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you will find the config file path was gave by -c argument", NewSimplePastToPastParticiple(), "you will find the config file path was given by -c argument")
}

func TestSimplePastToPastParticiple_CorrectBeBegan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ticket requires something from design before it can be began.", NewSimplePastToPastParticiple(), "Ticket requires something from design before it can be begun.")
}

func TestSimplePastToPastParticiple_CorrectBeingTook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Dunno, I saw some old threads about port not being took into account in asw-sdk library but seems fixed on aws side.", NewSimplePastToPastParticiple(), "Dunno, I saw some old threads about port not being taken into account in asw-sdk library but seems fixed on aws side.")
}

func TestSimplePastToPastParticiple_CorrectAreTook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the example provided, TP53 and LMNB1 genes are took as seeds.", NewSimplePastToPastParticiple(), "In the example provided, TP53 and LMNB1 genes are taken as seeds.")
}

func TestSimplePastToPastParticiple_CorrectIveWent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've went through some tutorials and went back and forth with AI translating programs from one language to the other.", NewSimplePastToPastParticiple(), "I've gone through some tutorials and went back and forth with AI translating programs from one language to the other.")
}

func TestSimplePastToPastParticiple_CorrectIveWentNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've went thru all the steps to help fix this Virus issue and im locked up.", NewSimplePastToPastParticiple(), "I've gone thru all the steps to help fix this Virus issue and im locked up.")
}

func TestSimplePastToPastParticiple_CorrectIdDid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'd did a calibration after the FW update now.", NewSimplePastToPastParticiple(), "I'd done a calibration after the FW update now.")
}

func TestSimplePastToPastParticiple_CorrectWeveWent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thanks for the feedback, but the issue is no longer relevant since we've went with different approach.", NewSimplePastToPastParticiple(), "Thanks for the feedback, but the issue is no longer relevant since we've gone with different approach.")
}

func TestSimplePastToPastParticiple_CorrectWedChose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "whatever number we'd chose, only one tab will be allowed to run", NewSimplePastToPastParticiple(), "whatever number we'd chosen, only one tab will be allowed to run")
}

func TestSimplePastToPastParticiple_CorrectYouveWrote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I love this project, it's impressing how many refactoring you've wrote in a limited amount of time.", NewSimplePastToPastParticiple(), "I love this project, it's impressing how many refactoring you've written in a limited amount of time.")
}

func TestSimplePastToPastParticiple_CorrectYouveRanNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "after youve ran it, execute the file_mover.ps1 using powershell", NewSimplePastToPastParticiple(), "after youve run it, execute the file_mover.ps1 using powershell")
}

func TestSimplePastToPastParticiple_CorrectYoudWrote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When I saw you'd wrote a terminal emulator I had to try it and so far it's amazing.", NewSimplePastToPastParticiple(), "When I saw you'd written a terminal emulator I had to try it and so far it's amazing.")
}

func TestSimplePastToPastParticiple_CorrectItsBroke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not sure why it's broke for me but not for you.", NewSimplePastToPastParticiple(), "Not sure why it's broken for me but not for you.")
}

func TestSimplePastToPastParticiple_CorrectItsBrokeNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Now its broke and won't do batch images (decoding error).", NewSimplePastToPastParticiple(), "Now its broken and won't do batch images (decoding error).")
}

func TestSimplePastToPastParticiple_CorrectTheyveBroke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've broke something again :D.", NewSimplePastToPastParticiple(), "They've broken something again :D.")
}

func TestSimplePastToPastParticiple_CorrectTheydForgot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "they found the process they'd forgot they were running", NewSimplePastToPastParticiple(), "they found the process they'd forgotten they were running")
}

func TestSimplePastToPastParticiple_DontFlagBeingWoke(t *testing.T) {
	linttest.AssertNoLints(t, "Being woke to gender discrimination is difficult", NewSimplePastToPastParticiple())
}

func TestSimplePastToPastParticiple_DontFlagBeWoke(t *testing.T) {
	linttest.AssertNoLints(t, "So You Want To Be Woke. The path to becoming woke is hard", NewSimplePastToPastParticiple())
}

func TestSimplePastToPastParticiple_DontFlagIdDid(t *testing.T) {
	linttest.AssertNoLints(t, "Prop id did not match.", NewSimplePastToPastParticiple())
}

func TestSimplePastToPastParticiple_DontFlagIdCame(t *testing.T) {
	linttest.AssertNoLints(t, "I'm a longtime user of UniFi and site ID came around after my account was established.", NewSimplePastToPastParticiple())
}

func TestSimplePastToPastParticiple_DontFlagIdWent(t *testing.T) {
	linttest.AssertNoLints(t, "Could not determine debug ID went away after cleaning the dist/ before the build, so that's unrelated.", NewSimplePastToPastParticiple())
}

func TestSimplePastToPastParticiple_DontFlagHaveLostIssue3011(t *testing.T) {
	linttest.AssertNoLints(t, "Elite Universities Have Lost Their Way", NewSimplePastToPastParticiple())
}

func TestSimplePastToPastParticiple_DontFlagHasLost(t *testing.T) {
	linttest.AssertNoLints(t, "He has lost his keys.", NewSimplePastToPastParticiple())
}

func TestSimplePastToPastParticiple_Alice(t *testing.T) {
	linttest.AssertNoLints(t, "it had struck her foot!", NewSimplePastToPastParticiple())
}

func TestSimplePastToPastParticiple_Gatsby(t *testing.T) {
	linttest.AssertNoLints(t, "the tuning-fork that had been struck upon a star", NewSimplePastToPastParticiple())
}
