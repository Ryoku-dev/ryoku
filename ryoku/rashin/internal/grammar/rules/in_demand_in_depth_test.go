package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInDemandInDepth_OfInDepthTechnicalDiscussion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One hour video call consisting of in depth technical discussion with lead developers", NewInDemandInDepth(), "One hour video call consisting of in-depth technical discussion with lead developers")
}

func TestInDemandInDepth_AMoreInDepthDocumentation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have added a more in depth documentation for each concept", NewInDemandInDepth(), "I have added a more in-depth documentation for each concept")
}

func TestInDemandInDepth_MostInDemandJobTechnologies(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Jupyter notebook for scraping and analysis of most in demand job technologies skills for data scientists", NewInDemandInDepth(), "Jupyter notebook for scraping and analysis of most in-demand job technologies skills for data scientists")
}

func TestInDemandInDepth_InvitedEngineerToExplainMoreInDepth(t *testing.T) {
	linttest.AssertSuggestionResult(t, "invited one of their engineers to the next meeting to explain this more in depth", NewInDemandInDepth(), "invited one of their engineers to the next meeting to explain this more in-depth")
}

func TestInDemandInDepth_VeryInDepthExplination(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a very in depth explination of naive bayes w.r.t implementation in python", NewInDemandInDepth(), "This is a very in-depth explination of naive bayes w.r.t implementation in python")
}

func TestInDemandInDepth_PrettyInDepthComparison(t *testing.T) {
	linttest.AssertSuggestionResult(t, "there should be a pretty in depth comparison of all of the options in the wiki", NewInDemandInDepth(), "there should be a pretty in-depth comparison of all of the options in the wiki")
}

func TestInDemandInDepth_QuiteInDepthAnalysis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There was already a quite in depth analysis for Tasmota here", NewInDemandInDepth(), "There was already a quite in-depth analysis for Tasmota here")
}
