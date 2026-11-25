package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestByOnesOwn_FixByHerOwn(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "If she is not managing by her own, it means that there is something wrong with either our documentation, or our error messages, or our ...", NewByOnesOwn(), []string{
		"If she is not managing on her own, it means that there is something wrong with either our documentation, or our error messages, or our ...",
		"If she is not managing by herself, it means that there is something wrong with either our documentation, or our error messages, or our ...",
	}, []string{})
}

func TestByOnesOwn_DontFlagByHerBreathing(t *testing.T) {
	linttest.AssertNoLints(t, "The silence of this deep section of the Spire was profound, broken only by her own ragged breathing.", NewByOnesOwn())
}

func TestByOnesOwn_FixByHisOwn(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "And make the hover info function creating the <title> tag by his own.", NewByOnesOwn(), []string{
		"And make the hover info function creating the <title> tag on his own.",
		"And make the hover info function creating the <title> tag by himself.",
	}, []string{})
}

func TestByOnesOwn_FixByHisOwnComma(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "it also makes a report all by his own, without touching anything", NewByOnesOwn(), []string{
		"it also makes a report all on his own, without touching anything",
		"it also makes a report all by himself, without touching anything",
	}, []string{})
}

func TestByOnesOwn_DontFlagByHisOwnEfforts(t *testing.T) {
	linttest.AssertNoLints(t, "My point is if someone needs jemalloc he gets it by his own efforts clearly realising why/when and so on.", NewByOnesOwn())
}

func TestByOnesOwn_FixByItsOwnInQuotes(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I set AdGuard DNS via Windows 11 settings, and I always found them to be changed \"by its own\".", NewByOnesOwn(), []string{
		"I set AdGuard DNS via Windows 11 settings, and I always found them to be changed \"on its own\".",
		"I set AdGuard DNS via Windows 11 settings, and I always found them to be changed \"by itself\".",
	}, []string{})
}

func TestByOnesOwn_FixByItsOwnAtEnd(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "The wheel starts speeding and spinning by its own.", NewByOnesOwn(), []string{
		"The wheel starts speeding and spinning on its own.",
		"The wheel starts speeding and spinning by itself.",
	}, []string{})
}

func TestByOnesOwn_DontFlagByItsOwnChanges(t *testing.T) {
	linttest.AssertNoLints(t, "Remix is spooked by its own changes", NewByOnesOwn())
}

func TestByOnesOwn_FixByMyOwn(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "how can I build a project by my own", NewByOnesOwn(), []string{
		"how can I build a project on my own",
		"how can I build a project by myself",
	}, []string{})
}

func TestByOnesOwn_DontFlagByMyOwnHr(t *testing.T) {
	linttest.AssertNoLints(t, "My colledge' ID is totally messed up by my own HR", NewByOnesOwn())
}

func TestByOnesOwn_FixByOurOwn(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "So, I think we should have the alternative BN implementation by our own.", NewByOnesOwn(), []string{
		"So, I think we should have the alternative BN implementation on our own.",
		"So, I think we should have the alternative BN implementation by ourselves.",
	}, []string{})
}

func TestByOnesOwn_DontFlagByOurOwnStandards(t *testing.T) {
	linttest.AssertNoLints(t, "Better by our own standards.", NewByOnesOwn())
}

func TestByOnesOwn_FixByTheirOwn(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Therefore, it would be great to allow the users to do the serialization by their own, rather than giving the full control to the SDK internally.", NewByOnesOwn(), []string{
		"Therefore, it would be great to allow the users to do the serialization on their own, rather than giving the full control to the SDK internally.",
		"Therefore, it would be great to allow the users to do the serialization by themselves, rather than giving the full control to the SDK internally.",
	}, []string{})
}

func TestByOnesOwn_DontFlagByTheirOwnBots(t *testing.T) {
	linttest.AssertNoLints(t, "GitHub needs a way to report issues and comments, even by their own bots.", NewByOnesOwn())
}

func TestByOnesOwn_FixByYourOwn(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "You should be able to see a new wallpaper slideshow created by your own.", NewByOnesOwn(), []string{
		"You should be able to see a new wallpaper slideshow created on your own.",
		"You should be able to see a new wallpaper slideshow created by yourself.",
	}, []string{})
}

func TestByOnesOwn_DontFlagByYourOwnLib(t *testing.T) {
	linttest.AssertNoLints(t, "Design with JSX, powered by your own component library.", NewByOnesOwn())
}

func TestByOnesOwn_DontFlagByItsOwnEndNoPunc(t *testing.T) {
	t.Skip("Upstream ignored: We can't detect the end of sentence when punctuation is missing.")
	linttest.AssertNoLints(t, "same infininte running and when i stop this node forcefully it show s the currect output dont know why it dont stop by its own i think its a bug", NewByOnesOwn())
}

func TestByOnesOwn_FixByYourOwnAnd(t *testing.T) {
	t.Skip("Upstream ignored: We might be able to handle this one")
	linttest.AssertGoodAndBadSuggestions(t, "After you install SPAdes by your own and add spades bin to the $PATH, ", NewByOnesOwn(), []string{
		"After you install SPAdes on your own and add spades bin to the $PATH, ",
		"After you install SPAdes by yourself and add spades bin to the $PATH, ",
	}, []string{})
}

func TestByOnesOwn_FixByYourOwnUsing(t *testing.T) {
	t.Skip("Upstream ignored: A construct that needs to be fixed that we can't handle yet")
	linttest.AssertGoodAndBadSuggestions(t, "I made some steps to this direction and currently you can implement it by your own using middleware", NewByOnesOwn(), []string{
		"I made some steps in this direction and currently you can implement it on your own using middleware",
		"I made some steps in this direction and currently you can implement it by yourself using middleware",
	}, []string{})
}

func TestByOnesOwn_FixByYourOwnPreposition(t *testing.T) {
	t.Skip("Upstream ignored: We might be able to handle this")
	linttest.AssertGoodAndBadSuggestions(t, "Of course feel free to add this by your own by pull request :)", NewByOnesOwn(), []string{
		"Of course feel free to add this on your own by pull request :)",
		"Of course feel free to add this by yourself by pull request :)",
	}, []string{})
}

func TestByOnesOwn_FixByYourOwnWe(t *testing.T) {
	t.Skip("Upstream ignored: Another construction we should fix but can't yet")
	linttest.AssertGoodAndBadSuggestions(t, "If you would like to develop this feature by your own we would be happy to help with this but", NewByOnesOwn(), []string{
		"If you would like to develop this feature on your own we would be happy to help with this but",
		"If you would like to develop this feature by yourself we would be happy to help with this but",
	}, []string{})
}

func TestByOnesOwn_FixByHerOwnDuring(t *testing.T) {
	t.Skip("Upstream ignored: See if it's always a legit error when the next word is a preposition")
	linttest.AssertGoodAndBadSuggestions(t, "most of the time she gets mad by her own during topics which can end her in an angry/annoyed mood", NewByOnesOwn(), []string{
		"most of the time she gets mad on her own during topics which can end her in an angry/annoyed mood",
		"most of the time she gets mad by herself during topics which can end her in an angry/annoyed mood",
	}, []string{})
}
