package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFleshOutVsFullFledged_FullFledgeHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Or do we want to become a full-fledge out-of-core ml library?", NewFleshOutVsFullFledged(), "Or do we want to become a full-fledged out-of-core ml library?")
}

func TestFleshOutVsFullFledged_FullFleshedSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Run a full fleshed ubuntu in termux without rooting your android.", NewFleshOutVsFullFledged(), "Run a full fledged ubuntu in termux without rooting your android.")
}

func TestFleshOutVsFullFledged_FullFleshedWebscraperHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A full-fleshed webscraper web app build on Next.js13 with tracking the prices of different product you want", NewFleshOutVsFullFledged(), "A full-fledged webscraper web app build on Next.js13 with tracking the prices of different product you want")
}

func TestFleshOutVsFullFledged_FullFleshedImplementationHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "almost provides a full-fleshed implementation allowing to read binary files into a tensor in a torchscript-compatible way.", NewFleshOutVsFullFledged(), "almost provides a full-fledged implementation allowing to read binary files into a tensor in a torchscript-compatible way.")
}

func TestFleshOutVsFullFledged_FullPledgedSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Any plan to make full pledged php server with swoole?", NewFleshOutVsFullFledged(), "Any plan to make full fledged php server with swoole?")
}

func TestFleshOutVsFullFledged_FullPledgedHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not yet, but I am considering the full-pledged snapshotting built-in.", NewFleshOutVsFullFledged(), "Not yet, but I am considering the full-fledged snapshotting built-in.")
}

func TestFleshOutVsFullFledged_NotFullyFledgedOut(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "There, it's not fully fledged out yet, but it's just an idea for now.", NewFleshOutVsFullFledged(), []string{
		"There, it's not fully fleshed out yet, but it's just an idea for now.",
		"There, it's not fully fledged yet, but it's just an idea for now.",
	}, []string{})
}

func TestFleshOutVsFullFledged_FullyFledgedOut(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Is the spawning process fully fledged out yet or am I joining the party too early?", NewFleshOutVsFullFledged(), []string{
		"Is the spawning process fully fleshed out yet or am I joining the party too early?",
		"Is the spawning process fully fledged yet or am I joining the party too early?",
	}, []string{})
}

func TestFleshOutVsFullFledged_FullyFleshedSpace(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "A Fully Fleshed E-Commerce web application, built with React, Redux and Firebase", NewFleshOutVsFullFledged(), []string{
		"A Fully Fledged E-Commerce web application, built with React, Redux and Firebase",
		"A Fully Fleshed out E-Commerce web application, built with React, Redux and Firebase",
	}, []string{})
}

func TestFleshOutVsFullFledged_FullyFleshedHyphen(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "This issue tracks the current progress towards publishing a fully-fleshed Fabric version of Gallery", NewFleshOutVsFullFledged(), []string{
		"This issue tracks the current progress towards publishing a fully-fleshed-out Fabric version of Gallery",
		"This issue tracks the current progress towards publishing a fully-fledged Fabric version of Gallery",
	}, []string{})
}

func TestFleshOutVsFullFledged_FullyPledgedSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Overall, we are already moving closer to having a fully pledged distributed recording and replay.", NewFleshOutVsFullFledged(), "Overall, we are already moving closer to having a fully fledged distributed recording and replay.")
}

func TestFleshOutVsFullFledged_FledgeOut(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For this we could fledge out the rule evaluation in a library and link this library to the server.", NewFleshOutVsFullFledged(), "For this we could flesh out the rule evaluation in a library and link this library to the server.")
}

func TestFleshOutVsFullFledged_FledgedOutSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We will be talking more about this when the technical details a more fledged out.", NewFleshOutVsFullFledged(), "We will be talking more about this when the technical details a more fleshed out.")
}
