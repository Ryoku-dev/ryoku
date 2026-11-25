package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPluralDecadesFourDigits_Eighties(t *testing.T) {
	linttest.AssertLintCount(t, "in the 1980's", NewPluralDecades(), 1)
}

func TestPluralDecadesFourDigits_Nineties(t *testing.T) {
	t.Skip("wip")
	linttest.AssertLintCount(t, "the 1990’s were a bit grungy", NewPluralDecades(), 1)
}

func TestPluralDecadesFourDigits_DontFlagThreeDigits(t *testing.T) {
	linttest.AssertLintCount(t, "200's doesn't look like a decade", NewPluralDecades(), 0)
}

func TestPluralDecadesFourDigits_DontFlagFiveDigits(t *testing.T) {
	linttest.AssertLintCount(t, "20000's doesn't look like a decade", NewPluralDecades(), 0)
}

func TestPluralDecadesFourDigits_DontFlagWithThousandsSeparator(t *testing.T) {
	linttest.AssertLintCount(t, "Nobody says \"in the 1,950's\".", NewPluralDecades(), 0)
}

func TestPluralDecadesFourDigits_DontFlagNotEndingWith0(t *testing.T) {
	linttest.AssertLintCount(t, "1977's best month was October", NewPluralDecades(), 0)
}

func TestPluralDecadesFourDigits_DontFlagAmbiguous1900sNppl(t *testing.T) {
	t.Skip("Too ambiguous to lint?")
	linttest.AssertNoLints(t, "star and fork 1900's gists by creating an account on GitHub.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixIn1900sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Children Aged 0-4 in 1900's Norway.", NewPluralDecades(), "Children Aged 0-4 in 1900s Norway.")
}

func TestPluralDecadesFourDigits_IgnoreHp1910s(t *testing.T) {
	t.Skip("Looks like a product name, not a decade")
	linttest.AssertNoLints(t, "Add support for HP 1910's", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixTheRoaring1920s(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "The \"Roaring 1920's\" wasn't just about the economy and technology.", NewPluralDecades(), "The \"Roaring 1920s\" wasn't just about the economy and technology.")
}

func TestPluralDecadesFourDigits_FixSpecial1920sTouch(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "It is beautiful and easily readable with that special 1920's touch.", NewPluralDecades(), "It is beautiful and easily readable with that special 1920s touch.")
}

func TestPluralDecadesFourDigits_FixInThe1920s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sir Josiah Stamp, president of the Bank of England and the second richest man in Britain in the 1920's, speaking at the University of Texas in 1927.", NewPluralDecades(), "Sir Josiah Stamp, president of the Bank of England and the second richest man in Britain in the 1920s, speaking at the University of Texas in 1927.")
}

func TestPluralDecadesFourDigits_DontFlagAmbiguous1950sNpsg(t *testing.T) {
	t.Skip("Grammar would be correct but the computer is from 1951 so must be a mistake for 1950s")
	linttest.AssertNoLints(t, "Simulator for 1950's MIT Whirlwind Computer.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixInThe1950s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Using the sandbox on the right, write and execute a query to return people born in the 1950's (1950 - 1959)", NewPluralDecades(), "Using the sandbox on the right, write and execute a query to return people born in the 1950s (1950 - 1959)")
}

func TestPluralDecadesFourDigits_FixAAdj1950sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Wave digital filter based emulation of a famous 1950's tube stereo limiter.", NewPluralDecades(), "Wave digital filter based emulation of a famous 1950s tube stereo limiter.")
}

func TestPluralDecadesFourDigits_Fix1950sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "1950's elevator randomly gets stuck", NewPluralDecades(), "1950s elevator randomly gets stuck")
}

func TestPluralDecadesFourDigits_FixFrom1950sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "documenting my family's camera business, from 1950's England, run by my father", NewPluralDecades(), "documenting my family's camera business, from 1950s England, run by my father")
}

func TestPluralDecadesFourDigits_FixFrom1950(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Plot the top ten most common baby names for New South Wales by year from the 1950's", NewPluralDecades(), "Plot the top ten most common baby names for New South Wales by year from the 1950s")
}

func TestPluralDecadesFourDigits_Fix1950sBrickBuiltHouse(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "We live in a 3 bedroom 1950's brick built house in the UK.", NewPluralDecades(), "We live in a 3 bedroom 1950s brick built house in the UK.")
}

func TestPluralDecadesFourDigits_FixAAdj1960sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Emulating a rare 1960's educational computer.", NewPluralDecades(), "Emulating a rare 1960s educational computer.")
}

func TestPluralDecadesFourDigits_IgnoreAmbiguous1960sNpsg(t *testing.T) {
	t.Skip("Ambiguous - could be referring to the specific year 1960 or the decade 1960s")
	linttest.AssertNoLints(t, "MyTheil - 1960's IP Sprint Hillarity.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_Fix1960sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Punchbag game inspired by 1960's TV Show Batman!", NewPluralDecades(), "Punchbag game inspired by 1960s TV Show Batman!")
}

func TestPluralDecadesFourDigits_IgnoreIn1960sAperture(t *testing.T) {
	t.Skip("ambiguous, not sure what it means")
	linttest.AssertNoLints(t, "Several \"SP entrances\" in 1960's Aperture have visible nodraw around entrance door", NewPluralDecades())
}

func TestPluralDecadesFourDigits_Fix1970sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "1970's chess engine CHEKMO-II + UCI adapter.", NewPluralDecades(), "1970s chess engine CHEKMO-II + UCI adapter.")
}

func TestPluralDecadesFourDigits_FixVprog1970sNpplDates(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "listsockets printing 1970's dates.", NewPluralDecades(), "listsockets printing 1970s dates.")
}

func TestPluralDecadesFourDigits_FixInA1970sStyleNpsg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I tried to create some catwalk in a 1970's style level.", NewPluralDecades(), "I tried to create some catwalk in a 1970s style level.")
}

func TestPluralDecadesFourDigits_Fix1970sPongGame(t *testing.T) {
	t.Skip("Grammar would be correct but Pong is from 1972 so must be a mistake for 1970s")
	linttest.AssertSuggestionResult(t, "1970's Pong game rewritten in C++.", NewPluralDecades(), "1970s Pong game rewritten in C++.")
}

func TestPluralDecadesFourDigits_Fix1970sInParens(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Convert a MIDI file to a record compatible with vintage (1970's) Fisher Price music box record players", NewPluralDecades(), "Convert a MIDI file to a record compatible with vintage (1970s) Fisher Price music box record players")
}

func TestPluralDecadesFourDigits_FixInThe1970s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "may have begun, depending on when you start counting, in the 1970's.", NewPluralDecades(), "may have begun, depending on when you start counting, in the 1970s.")
}

func TestPluralDecadesFourDigits_IgnoreUsernameHyphen1970sGists(t *testing.T) {
	linttest.AssertNoLints(t, "GitHub Gist: star and fork ricardo-reis-1970's gists by creating an account on GitHub.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixAfter1970sNpplMinicomputers(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "I'm also working on extending it as my CPU is modelled after 1970's minicomputers", NewPluralDecades(), "I'm also working on extending it as my CPU is modelled after 1970s minicomputers")
}

func TestPluralDecadesFourDigits_FixOfThe1970s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I despise both as outdated, hard to use relics of the 1970's.", NewPluralDecades(), "I despise both as outdated, hard to use relics of the 1970s.")
}

func TestPluralDecadesFourDigits_FixCouplesInThe1970s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This visualization tracks a sample of couples in the 1970's to show how long they transition through relationship stages.", NewPluralDecades(), "This visualization tracks a sample of couples in the 1970s to show how long they transition through relationship stages.")
}

func TestPluralDecadesFourDigits_FixDevelopedInEarly1970s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "GPS, originally developed in the early 1970's, is a unidirectional (broadcast only) system", NewPluralDecades(), "GPS, originally developed in the early 1970s, is a unidirectional (broadcast only) system")
}

func TestPluralDecadesFourDigits_FixFromThe1980sLike(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Old Stern tables from the 1980's like Flight 2000, Catacomb, etc. are playing audio samples twice, it seems.", NewPluralDecades(), "Old Stern tables from the 1980s like Flight 2000, Catacomb, etc. are playing audio samples twice, it seems.")
}

func TestPluralDecadesFourDigits_FixItsThe1980s(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Declarative Rapid Application Development like it's the 1980's again.", NewPluralDecades(), "Declarative Rapid Application Development like it's the 1980s again.")
}

func TestPluralDecadesFourDigits_FixFromThe1980sEnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Former countries from the 1980's", NewPluralDecades(), "Former countries from the 1980s")
}

func TestPluralDecadesFourDigits_FixA1980sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "A re-imiplementation of a classic 1980's DOS game, but in D.", NewPluralDecades(), "A re-imiplementation of a classic 1980s DOS game, but in D.")
}

func TestPluralDecadesFourDigits_FixOfThe1980s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Pugputer is a little labor of love, made as a tribute to the early home computers of the 1980's.", NewPluralDecades(), "The Pugputer is a little labor of love, made as a tribute to the early home computers of the 1980s.")
}

func TestPluralDecadesFourDigits_FixOfThe1980sNpsg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "FPGA implementation of the 1980's \"Music 5000\" wavetable synthesiser", NewPluralDecades(), "FPGA implementation of the 1980s \"Music 5000\" wavetable synthesiser")
}

func TestPluralDecadesFourDigits_FixBasedOffOfThe1980sNpsg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Space Fortress is based off of the 1980's vector-based arcade game by Cinematronics called Star Castle.", NewPluralDecades(), "Space Fortress is based off of the 1980s vector-based arcade game by Cinematronics called Star Castle.")
}

func TestPluralDecadesFourDigits_IgnoreAmbiguous1980s(t *testing.T) {
	t.Skip("Ambiguous - could be referring to the specific year 1980 or the decade 1980s")
	linttest.AssertNoLints(t, "1980's Old aperture coop checkpoint uses the timer signage instead of checkmarks.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_Fix1980sNppl(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "A project resurrecting the classic 1980's Usborne Computer Guide books, for a new generation of programmers.", NewPluralDecades(), "A project resurrecting the classic 1980s Usborne Computer Guide books, for a new generation of programmers.")
}

func TestPluralDecadesFourDigits_FixTheEndOfMissingDeterminer1980s(t *testing.T) {
	t.Skip("Missing determiner is out of the scope of the current version of this linter")
	linttest.AssertSuggestionResult(t, "System software for TIM011, a school computer from the end of 1980's made in former Yugoslavia", NewPluralDecades(), "System software for TIM011, a school computer from the end of the 1980s made in former Yugoslavia")
}

func TestPluralDecadesFourDigits_FixInThe1980sFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "HMSL was originally released in the 1980's for Mac Plus and Amiga", NewPluralDecades(), "HMSL was originally released in the 1980s for Mac Plus and Amiga")
}

func TestPluralDecadesFourDigits_FixTheAdj1980sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Modern remake of Pole Position, the classic 1980's arcade racing game from Atari.", NewPluralDecades(), "Modern remake of Pole Position, the classic 1980s arcade racing game from Atari.")
}

func TestPluralDecadesFourDigits_FixSinceThe1980s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Since the 1980's the most common way to interact with a computer is via the graphical user interface (GUI)", NewPluralDecades(), "Since the 1980s the most common way to interact with a computer is via the graphical user interface (GUI)")
}

func TestPluralDecadesFourDigits_FixTheAdj1990sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "42 3d Graphics Project, recreating the classic 1990's game Wolfienstien 3d", NewPluralDecades(), "42 3d Graphics Project, recreating the classic 1990s game Wolfienstien 3d")
}

func TestPluralDecadesFourDigits_FixA1990sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "A 1990's Retro linux-rice for Hyprland or Sway, based on Quickshell.", NewPluralDecades(), "A 1990s Retro linux-rice for Hyprland or Sway, based on Quickshell.")
}

func TestPluralDecadesFourDigits_LacksDeterminerStuckIn1990s(t *testing.T) {
	t.Skip("Missing determiner is out of the scope of the current version of this linter")
	linttest.AssertSuggestionResult(t, "Docs are stuck in 1990's - need AWS or Azure example", NewPluralDecades(), "Docs are stuck in the 1990s - need AWS or Azure example")
}

func TestPluralDecadesFourDigits_FixThe1990sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "This program recreates the 1990's arcade game \"Boulder Dash.\"", NewPluralDecades(), "This program recreates the 1990s arcade game \"Boulder Dash.\"")
}

func TestPluralDecadesFourDigits_FixInThe1990sComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the 1990's, Innovative Computer Solutions released multiple programs for the Newton MessagePad as shareware", NewPluralDecades(), "In the 1990s, Innovative Computer Solutions released multiple programs for the Newton MessagePad as shareware")
}

func TestPluralDecadesFourDigits_FixAMid1990sNpsg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This repository is a modernization of a mid 1990's implementation of the ZMODEM protocol called 'zmtx-zmrx'.", NewPluralDecades(), "This repository is a modernization of a mid 1990s implementation of the ZMODEM protocol called 'zmtx-zmrx'.")
}

func TestPluralDecadesFourDigits_LacksDeterminerWrittenInJavaIn1990s(t *testing.T) {
	t.Skip("Missing determiner is out of the scope of the current version of this linter")
	linttest.AssertSuggestionResult(t, "JMud, mud server written in Java in 1990's.", NewPluralDecades(), "JMud, mud server written in Java in the 1990s.")
}

func TestPluralDecadesFourDigits_FixAAdj1990sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Port of a famous 1990's fighting game to MSX2.", NewPluralDecades(), "Port of a famous 1990s fighting game to MSX2.")
}

func TestPluralDecadesFourDigits_FixCirca1990s(t *testing.T) {
	t.Skip("circa 1990s doesn't sound like natural English. changing to 'circa the 1990s' is out of scope for this linter")
	linttest.AssertSuggestionResult(t, "Inspired by Digimon \"Digivices\" tamagotchis circa 1990's.", NewPluralDecades(), "Inspired by Digimon \"Digivices\" tamagotchis circa the 1990s.")
}

func TestPluralDecadesFourDigits_FixFor1990sNppl(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Daughter-board for reprogramming 1990's Toyota ECUs", NewPluralDecades(), "Daughter-board for reprogramming 1990s Toyota ECUs")
}

func TestPluralDecadesFourDigits_FixThe1990sClassicMarioHit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A remake of the 1990's classic mario hit.", NewPluralDecades(), "A remake of the 1990s classic mario hit.")
}

func TestPluralDecadesFourDigits_FixDevelopedInEarly1990s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The code was originally developed in the early 1990's", NewPluralDecades(), "The code was originally developed in the early 1990s")
}

func TestPluralDecadesFourDigits_Fix2000sStyle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "2000's-style media library for vintage cellphones (Nokia, etc.)", NewPluralDecades(), "2000s-style media library for vintage cellphones (Nokia, etc.)")
}

func TestPluralDecadesFourDigits_IgnoreForkUsernameHyphen2000sNppl(t *testing.T) {
	linttest.AssertNoLints(t, "star and fork vishal-2000's gists by creating an account on GitHub.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixInThe2000s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Simulator engine for reproducing LCD games made by McDonald's in the 2000's.", NewPluralDecades(), "Simulator engine for reproducing LCD games made by McDonald's in the 2000s.")
}

func TestPluralDecadesFourDigits_FixInTheEarly2000sMissingDeterminer(t *testing.T) {
	t.Skip("Missing determiner is out of the scope of the current version of this linter")
	linttest.AssertSuggestionResult(t, "Silo was originally released in early 2000's using LLNL-home-grown license verbiage.", NewPluralDecades(), "Silo was originally released in the early 2000s using LLNL-home-grown license verbiage.")
}

func TestPluralDecadesFourDigits_IgnoreViewUsernameHyphen2000sNpsg(t *testing.T) {
	linttest.AssertNoLints(t, "View lxw-2000's full-sized avatar.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixEarly2000sStyleNpsg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Early 2000's Style Personal Webpage.", NewPluralDecades(), "Early 2000s Style Personal Webpage.")
}

func TestPluralDecadesFourDigits_FixFromTheMid2000s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Modeled after the now-defunct Geosense game from the mid 2000's", NewPluralDecades(), "Modeled after the now-defunct Geosense game from the mid 2000s")
}

func TestPluralDecadesFourDigits_FixForTheEarly2000sGames(t *testing.T) {
	linttest.AssertSuggestionResult(t, "GothicKit is a community-run organization hosting libraries and tools for the early 2000's games Gothic and Gothic II.", NewPluralDecades(), "GothicKit is a community-run organization hosting libraries and tools for the early 2000s games Gothic and Gothic II.")
}

func TestPluralDecadesFourDigits_FixBackInThe2000s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A basic music player/organizer I created back in the 2000's - carderne/CAMO.", NewPluralDecades(), "A basic music player/organizer I created back in the 2000s - carderne/CAMO.")
}

func TestPluralDecadesFourDigits_FixMid2000s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Things I wrote about RDF from the mid-2000's.", NewPluralDecades(), "Things I wrote about RDF from the mid-2000s.")
}

func TestPluralDecadesFourDigits_IgnoreAuthor2010sPublicationReference(t *testing.T) {
	t.Skip("Sinnemäki 2010 here refers to the author's publication from 2010")
	linttest.AssertNoLints(t, "CLDF version of Sinnemäki 2010's dataset on zero marking and word order", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreBazelCallsVs2010sCl(t *testing.T) {
	t.Skip("Looks like a product name, not a decade")
	linttest.AssertNoLints(t, "Bazel calls VS 2010's cl with /DEBUG:FASTLINK", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixSinceTheEarly2010s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the degree of drop-off of CouchDB online community activity since the early-2010's NoSQL craze faded", NewPluralDecades(), "the degree of drop-off of CouchDB online community activity since the early-2010s NoSQL craze faded")
}

func TestPluralDecadesFourDigits_FixBlogPostsOfThe2010s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's jumped off the esoteric analytics blog posts of the 2010's and on to your television screens and into your video games", NewPluralDecades(), "It's jumped off the esoteric analytics blog posts of the 2010s and on to your television screens and into your video games")
}

func TestPluralDecadesFourDigits_IgnoreAmbiguous2020s(t *testing.T) {
	t.Skip("Ambiguous. Looks like awkward wording for `the IEEE CEC's 2020 Strategy Card Game AI Competition")
	linttest.AssertNoLints(t, "This is a bot for Legends of Code and Magic submitted to the IEEE CEC 2020's Strategy Card Game AI Competition.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreAmbiguous2020s2(t *testing.T) {
	t.Skip("Ambiguous. Maybe awkward wording for `CSSPI Fall 2020's frontend`??")
	linttest.AssertNoLints(t, "CSSPI Fall 2020's frontend mobile web application utilizing React Native.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreAmbiguous2020s2024s(t *testing.T) {
	t.Skip("A human can tell from the comparison to `2024's` that `2020's` refers to a specific year/version/release. Harper has no way to tell.")
	linttest.AssertNoLints(t, "App that converts MSFS 2020's DDS texture format to MSFS 2024's KTX2 format", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreErum2020s(t *testing.T) {
	t.Skip("eRum 2020 is probably an event name, not a decade")
	linttest.AssertNoLints(t, "A repository for purposes of eRum 2020's workshop \"Image processing and computer vision with R\", held on Saturday, June 20, 2020.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreAmbiguous2020s3(t *testing.T) {
	t.Skip("Ambiguous. Not sure what it means")
	linttest.AssertNoLints(t, "Crashing upon loading saved game in 2020's", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreDefcon2020sEvent(t *testing.T) {
	t.Skip("DEF CON 2020's is probably an event name, not a decade")
	linttest.AssertNoLints(t, "Reimplementation of DEF CON 2020's Pinboool Binary - pinboool.py.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreUsernameHyphen2020sAvatar(t *testing.T) {
	linttest.AssertNoLints(t, "View theredpill-2020's full-sized avatar.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreAmbiguous2020sScipy(t *testing.T) {
	t.Skip("Ambiguous. Not sure what it means")
	linttest.AssertNoLints(t, "No imread() in 2020's Scipy v1.5.x.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnorePullInto2020s(t *testing.T) {
	t.Skip("ambiguous")
	linttest.AssertNoLints(t, "Pull into the 2020's.", NewPluralDecades())
}

func TestPluralDecadesFourDigits_IgnoreBlasasEtAl2020s(t *testing.T) {
	t.Skip("Odd. Looks like it should be `Blasas et al's 2020 \"FUT9-driven ...\"`")
	linttest.AssertNoLints(t, "Scripts that were used to create figure 5 in Blasas et al. 2020's \"FUT9-driven programming of colon cancer cells towards a stem cell-like state\"", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixInThe1990sAndThe2000s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "NTXShape, a converter I developed in the 1990's and maintained through the 2000's", NewPluralDecades(), "NTXShape, a converter I developed in the 1990s and maintained through the 2000s")
}

func TestPluralDecadesFourDigits_FixEarly2000sEarly1990sEarly1980s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "CDISC since 2005, XML since the early 2000's, @SAS since the early 1990's, Programming since the early 1980's.", NewPluralDecades(), "CDISC since 2005, XML since the early 2000s, @SAS since the early 1990s, Programming since the early 1980s.")
}

func TestPluralDecadesFourDigits_FixDatesInThe1960sComma1950sEarly1900s(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "OK for dates in the 1960's, 1950's... Now... I expect 1939-12-31T23 ... Was Belgium ever not in UTC+1 timezone in early 1900's ?", NewPluralDecades(), "OK for dates in the 1960s, 1950s... Now... I expect 1939-12-31T23 ... Was Belgium ever not in UTC+1 timezone in early 1900s ?")
}

func TestPluralDecadesFourDigits_FixLate1970sEarly1980s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Late 1970's/Early 1980's Text Adventure Game from the Mainframe era", NewPluralDecades(), "Late 1970s/Early 1980s Text Adventure Game from the Mainframe era")
}

func TestPluralDecadesFourDigits_FixInThe1970sAndEarly1980s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We modeled the gas mileage of 398 cars built in the 1970's and early 1980's", NewPluralDecades(), "We modeled the gas mileage of 398 cars built in the 1970s and early 1980s")
}

func TestPluralDecadesFourDigits_FixFromTheLate1970sEarly1980s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Europe Card Bus (ECB) is a Retro CPU Bus standard from the late 1970's / early 1980's.", NewPluralDecades(), "Europe Card Bus (ECB) is a Retro CPU Bus standard from the late 1970s / early 1980s.")
}

func TestPluralDecadesFourDigits_IgnoreOn80sOn90sOn2000sOn2010sOn2020s(t *testing.T) {
	t.Skip("Unnatural English where 'on [decade]' should be 'in the [decade]'")
	linttest.AssertNoLints(t, "Developer on 80's, software engineer on 90's, tech lead on 2000's, manager from 2010's on and CIO/CDO/CTO on 2020's", NewPluralDecades())
}

func TestPluralDecadesFourDigits_FixInThe80sInThe90s2000s2010s(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "UUCP BBS in the 80's Winternet / ABX Net Un Hacking in the 90's FOU 2000's CIS Inc. 2010's For Every Hacker there is an Equal and Opposite Hacker.", NewPluralDecades(), "UUCP BBS in the 80s Winternet / ABX Net Un Hacking in the 90s FOU 2000s CIS Inc. 2010s For Every Hacker there is an Equal and Opposite Hacker.")
}

func TestPluralDecadesFourDigits_FixInThe1990sAnd2000s(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Edelman and his colleagues in the 1990's and 2000's.", NewPluralDecades(), "Edelman and his colleagues in the 1990s and 2000s.")
}

func TestPluralDecadesFourDigits_Fix1930sAmpersand1940s(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "DJ and collector of quality tango music from the golden era (1930's & 1940's)", NewPluralDecades(), "DJ and collector of quality tango music from the golden era (1930s & 1940s)")
}

func TestPluralDecadesFourDigits_Fix80s90s2000s2020s(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Nerdy coder-kid on ZX81 and Oric 1 in the 80's, nerdy teen-tech writer on PC in 90's, nerdy engineer in the 2000's, 2020's MongoDB addict, Databricks fan now!", NewPluralDecades(), "Nerdy coder-kid on ZX81 and Oric 1 in the 80s, nerdy teen-tech writer on PC in 90s, nerdy engineer in the 2000s, 2020s MongoDB addict, Databricks fan now!")
}
