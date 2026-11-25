package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDigUnderTheHood_DigHood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But then when we actually like, you know, dig under the hood, what we see so much is that, you know, companies are reporting that they're not seeing the return on investment that they expected from generative AI when you have so many investment dollars already bound up in this vision.", NewDigUnderTheHood(), "But then when we actually like, you know, look under the hood, what we see so much is that, you know, companies are reporting that they're not seeing the return on investment that they expected from generative AI when you have so many investment dollars already bound up in this vision.")
}

func TestDigUnderTheHood_DiggingHood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But then when we actually like, you know, dig under the hood, what we see so much is that, you know, companies are reporting that they're not seeing the return on investment that they expected from generative AI when you have so many investment dollars already bound up in this vision.", NewDigUnderTheHood(), "But then when we actually like, you know, look under the hood, what we see so much is that, you know, companies are reporting that they're not seeing the return on investment that they expected from generative AI when you have so many investment dollars already bound up in this vision.")
}

func TestDigUnderTheHood_DigsHood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But Ben digs under the hood and the underlying metrics still look real: same attack angle, same bat speed, still hitting the ball hard.", NewDigUnderTheHood(), "But Ben looks under the hood and the underlying metrics still look real: same attack angle, same bat speed, still hitting the ball hard.")
}

func TestDigUnderTheHood_DugHood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Love that you dug under the hood to turn off output capture to figure out where things were hanging up.", NewDigUnderTheHood(), "Love that you looked under the hood to turn off output capture to figure out where things were hanging up.")
}

func TestDigUnderTheHood_DigBonnet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Dig under the bonnet of that crude metric and you'll find Armagh, Galway, Donegal and Kerry have panels built to last 70 minutes.", NewDigUnderTheHood(), "Look under the bonnet of that crude metric and you'll find Armagh, Galway, Donegal and Kerry have panels built to last 70 minutes.")
}

func TestDigUnderTheHood_DiggingBonnet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When your days are consumed with writing, analysing data and digging under the bonnet of other people's businesses to find the award-winning gold, ...", NewDigUnderTheHood(), "When your days are consumed with writing, analysing data and looking under the bonnet of other people's businesses to find the award-winning gold, ...")
}

func TestDigUnderTheHood_DugBonnet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Don't know if anyone has dug under the bonnet yet, but any opinions as to whether .htaccess support is something that could be achieved via a patch or plugin?", NewDigUnderTheHood(), "Don't know if anyone has looked under the bonnet yet, but any opinions as to whether .htaccess support is something that could be achieved via a patch or plugin?")
}
