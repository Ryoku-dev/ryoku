package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTheMy_CorrectTheMyAtomicLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the my", NewTheMy(), "my")
}

func TestTheMy_CorrectTheMyAtomic2ndSuggestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the my", NewTheMy(), "the")
}

func TestTheMy_CorrectTheMyAtomicUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The my", NewTheMy(), "My")
}

func TestTheMy_CorrectMyTheAtomicLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "my the", NewTheMy(), "my")
}

func TestTheMy_CorrectMyTheAtomic2ndSuggestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "my the", NewTheMy(), "the")
}

func TestTheMy_CorrectMyTheAtomicUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My the", NewTheMy(), "My")
}

func TestTheMy_DontCorrectCapitalizedPossessive(t *testing.T) {
	linttest.AssertLintCount(t, "For some time the My Projects personal page was \"sluggish\" or took some time to generate the miniature depicting the project, now it seems completely stuck ...\n", NewTheMy(), 0)
}

func TestTheMy_CorrectTheMyGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When I try to configure the my react-native app to support koltin file, this library gives these errors", NewTheMy(), "When I try to configure my react-native app to support koltin file, this library gives these errors")
}

func TestTheMy_CorrectTheOurGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Source codes of the our paper titled \"Multi-level Textual-Visual Alignment and Fusion Network for Multimodal Aspect-based Sentiment Analysis\"", NewTheMy(), "Source codes of our paper titled \"Multi-level Textual-Visual Alignment and Fusion Network for Multimodal Aspect-based Sentiment Analysis\"")
}

func TestTheMy_CorrectTheTheirGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the slider cannot render when i use again the their component on NextJS app", NewTheMy(), "the slider cannot render when i use again their component on NextJS app")
}

func TestTheMy_CorrectYourTheGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This plugin allows you to view your the information about order and customer from your spree store on zendesk", NewTheMy(), "This plugin allows you to view your information about order and customer from your spree store on zendesk")
}

func TestTheMy_CorrectMyTheGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Scripts used my the project to collect, process and store social media data from a number of sources", NewTheMy(), "Scripts used my project to collect, process and store social media data from a number of sources")
}

func TestTheMy_DontCorrectTheYourGithub(t *testing.T) {
	linttest.AssertLintCount(t, "What exactly is the sort order of list names on the Your Stars page?", NewTheMy(), 0)
}

func TestTheMy_DontCorrectMyTheGithub(t *testing.T) {
	linttest.AssertLintCount(t, "My The Frame TV is not pulling information properly", NewTheMy(), 0)
}

func TestTheMy_CorrectOurTheGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Companion Repository to our the whitepaper \"Towards Reliable and Scalable Linux Kernel CVE Attribution in Automated Static Firmware Analyses\"", NewTheMy(), "Companion Repository to our whitepaper \"Towards Reliable and Scalable Linux Kernel CVE Attribution in Automated Static Firmware Analyses\"")
}

func TestTheMy_CorrectTheirTheGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Types exported by @_exported remember only their the original module", NewTheMy(), "Types exported by @_exported remember only their original module")
}

func TestTheMy_DontCorrectHerTheGithub(t *testing.T) {
	linttest.AssertLintCount(t, "Create an admin role for boba-tan and give her the GoreMaster role only in !gore", NewTheMy(), 0)
}

func TestTheMy_CorrectTheHisGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Allows the user to specify the his last name.", NewTheMy(), "Allows the user to specify his last name.")
}

func TestTheMy_CorrectHisTheGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One interesting creation was his the Schelling segregation model", NewTheMy(), "One interesting creation was his Schelling segregation model")
}

func TestTheMy_CorrectTheHerGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In memory of the occasion when our Queen Victoria graciously came to see our Island, and the her Royal Consort Albert landed at Ramsey", NewTheMy(), "In memory of the occasion when our Queen Victoria graciously came to see our Island, and her Royal Consort Albert landed at Ramsey")
}
