package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPluralDecadesTwoDigits_Eighties(t *testing.T) {
	linttest.AssertLintCount(t, "in the 80's", NewPluralDecades(), 1)
}

func TestPluralDecadesTwoDigits_Nineties(t *testing.T) {
	t.Skip("wip")
	linttest.AssertLintCount(t, "the 90’s were a bit grungy", NewPluralDecades(), 1)
}

func TestPluralDecadesTwoDigits_DontFlagThreeDigits(t *testing.T) {
	linttest.AssertNoLints(t, "200's doesn't look like a decade", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagOneDigit(t *testing.T) {
	linttest.AssertNoLints(t, "0's doesn't look like a decade", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagNotEndingWith0(t *testing.T) {
	linttest.AssertNoLints(t, "'77's best month was October", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagDotVersionNumbers(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "A bug is apparently in FOG 1.5.10's normalize() function inside init.xz.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_FixShowingThe10sOfHours(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "It took 10's of hours to debug this issue", NewPluralDecades(), "It took 10s of hours to debug this issue")
}

func TestPluralDecadesTwoDigits_DontFlagWindows10(t *testing.T) {
	linttest.AssertNoLints(t, "How about Windows 10's taskbar progress bar?", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagSpaceVersionNumbersResharper10(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "\"gd\" doesn't work correctly with ReSharper 10's \"Usage-aware Go to Declaration\"", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagSpaceVersionNumbersMermaid10(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "mermaid 10's ESM only support breaks compat with many apps", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagNpm10sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "Align npm packages to npm 10's node engine range", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagXcode10sVersionNumber(t *testing.T) {
	linttest.AssertNoLints(t, "Leverage Xcode 10's new \"File list\" feature for input/output files of Run Script build phases", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagModo10sVersionNumber(t *testing.T) {
	t.Skip("non-well-known products couldn't really be checked for though")
	linttest.AssertNoLints(t, "Modo 10's Unreal editor plugin (for loading PBR materials / textures)", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagWindows10sTouchKeyboard(t *testing.T) {
	linttest.AssertNoLints(t, "Arrow Key Command History Navigation Not Working Using Windows 10's Built-in 'Touch Keyboard'", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagAndroid10sScopedStorage(t *testing.T) {
	linttest.AssertNoLints(t, "Android 10's Scoped storage using Image picker (Gallery / Camera) with compression example.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagWindows10sOpenssh(t *testing.T) {
	linttest.AssertNoLints(t, "If I try to set Windows 10's OpenSSH ssh-agent.exe as the pageant executable, I get an error message", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagNode10sResolutionAlgorithm(t *testing.T) {
	t.Skip("release/version number?")
	linttest.AssertNoLints(t, "node10 encoded Node.js 10's resolution algorithm, which predates ESM support", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagXcode10sNewBuildSystem(t *testing.T) {
	linttest.AssertNoLints(t, "Fixes the third party dependency issues introduced by Xcode 10's new build system.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagWindows10sControlledFolderAccess(t *testing.T) {
	linttest.AssertNoLints(t, "NVDA install fails when Windows 10's Controlled Folder Access is enabled", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagWindows10sWsl(t *testing.T) {
	linttest.AssertNoLints(t, "By default Windows 10's WSL has trouble opening paths on mounted VeraCrypt volumes.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagCpp20sStdSpan(t *testing.T) {
	linttest.AssertNoLints(t, "This repository contains a single-header implementation of C++20's std::span, conforming to the C++20 committee draft.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagSpaceVersionNumbersVirtualenv20(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "Clarifying virtualenv 20's -p behavior", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagHyphenatedVersionNumbersSoi20(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "View soi-20's full-sized avatar.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagCpp20sConcepts(t *testing.T) {
	linttest.AssertNoLints(t, "Replace SFINAE with C++20's Concepts and Constraints", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagCpp20sStdLatch(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "As part of an experiment I recently switched from ducc's latch class to C++20's std::latch, and to my surprise I noticed a significant speedup", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagTeam20sApplication(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "Team 20's application for the 2020 Teens In AI Global COVID Hackathon.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagLesson20sSqlQuery(t *testing.T) {
	t.Skip("should we detect lesson numbers?")
	linttest.AssertNoLints(t, "Lesson 20's SQL Query is too inefficient", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagCpp20sInitializationChange(t *testing.T) {
	linttest.AssertNoLints(t, "Potential issue with C++20's initialization change.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_FixMy20s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just a software engineer in his 20's.", NewPluralDecades(), "Just a software engineer in his 20s.")
}

func TestPluralDecadesTwoDigits_FixMyEarly20s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thank you Steve Wozniak :-) it was the dream machine of my early 20's.", NewPluralDecades(), "Thank you Steve Wozniak :-) it was the dream machine of my early 20s.")
}

func TestPluralDecadesTwoDigits_FixMyLate20s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I only decided that I wanted to work in the field at my late 20's when I chose a graduation course", NewPluralDecades(), "I only decided that I wanted to work in the field at my late 20s when I chose a graduation course")
}

func TestPluralDecadesTwoDigits_DontFlagSdkVersions(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "binder: We call SDK 30's bindServiceAsUser() and SDK 26's bindDeviceAdminServiceAsUser() methods without a runtime check", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlagJxnHyphen30sUsername(t *testing.T) {
	t.Skip("wip")
	linttest.AssertNoLints(t, "GitHub Gist: star and fork jxn-30's gists by creating an account on GitHub", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_FixMy30s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm a developer in my 30's.", NewPluralDecades(), "I'm a developer in my 30s.")
}

func TestPluralDecadesTwoDigits_FixMyMid30s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Today, in my mid 30's, I am proficient in a wide range of programming languages and environments.", NewPluralDecades(), "Today, in my mid 30s, I am proficient in a wide range of programming languages and environments.")
}

func TestPluralDecadesTwoDigits_FixMyEarly30s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Software Developer in my early 30's.", NewPluralDecades(), "Software Developer in my early 30s.")
}

func TestPluralDecadesTwoDigits_DontFlagGroup40s(t *testing.T) {
	t.Skip("might be too ambiguous to detect?")
	linttest.AssertNoLints(t, "Group 40's team maths game.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_FixMy40s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm a married father of two in my 40's who currently programs at work by day, and, well, at home by night.", NewPluralDecades(), "I'm a married father of two in my 40s who currently programs at work by day, and, well, at home by night.")
}

func TestPluralDecadesTwoDigits_FixTheir40s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "for a person in their 40's you're awfully bitter", NewPluralDecades(), "for a person in their 40s you're awfully bitter")
}

func TestPluralDecadesTwoDigits_FixMyMid40s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am a system developer in my mid 40's working in the health care sector at DNV Imatis AS.", NewPluralDecades(), "I am a system developer in my mid 40s working in the health care sector at DNV Imatis AS.")
}

func TestPluralDecadesTwoDigits_FixTheirMid40s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "even my parents who are in their mid-40's can manage to use the default interface", NewPluralDecades(), "even my parents who are in their mid-40s can manage to use the default interface")
}

func TestPluralDecadesTwoDigits_DontFlag50sUsername(t *testing.T) {
	t.Skip("here it's a username but Harper has no way to know")
	linttest.AssertNoLints(t, "View 50's full-sized avatar.", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_DontFlag60Seconds(t *testing.T) {
	t.Skip("here it means 60+ seconds")
	linttest.AssertNoLints(t, "WSL cold startup 60's +", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_FixMyLate60s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Comment: i'm a white woman in my late 60's and believe me, they are not too crazy about me", NewPluralDecades(), "Comment: i'm a white woman in my late 60s and believe me, they are not too crazy about me")
}

func TestPluralDecadesTwoDigits_DontFlagDotnetRuntime70s(t *testing.T) {
	t.Skip("ambiguous: version number?")
	linttest.AssertNoLints(t, "dotnet-runtime-70's release of 16th of May is causing \"version `GLIBC_2.34' not found\"", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_FixLateHyphen70s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Retrocomputer built from late-70's TTL logic chips", NewPluralDecades(), "Retrocomputer built from late-'70s TTL logic chips")
}

func TestPluralDecadesTwoDigits_FixAFun70sIndustrial(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "A fun 70's industrial \"launch\" control panel with a Yubikey key switch", NewPluralDecades(), "A fun '70s industrial \"launch\" control panel with a Yubikey key switch")
}

func TestPluralDecadesTwoDigits_FixOfThe80sNpsg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A reboot of the 80's Microwriter accessible chord keyboard done using an Arduino.", NewPluralDecades(), "A reboot of the '80s Microwriter accessible chord keyboard done using an Arduino.")
}

func TestPluralDecadesTwoDigits_FixAn80sNpsg(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "A remake of an 80's card game classic \"Around the World\"", NewPluralDecades(), "A remake of an '80s card game classic \"Around the World\"")
}

func TestPluralDecadesTwoDigits_FixThe80sNpsg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Small remake of the 80's legendary paperboy arcade game", NewPluralDecades(), "Small remake of the '80s legendary paperboy arcade game")
}

func TestPluralDecadesTwoDigits_FixThe80sStyleGameBreakout(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I called this pong but then was reminded that it more closely resembles the 80's style game Breakout.", NewPluralDecades(), "I called this pong but then was reminded that it more closely resembles the '80s style game Breakout.")
}

func TestPluralDecadesTwoDigits_FixThe80sMicrowriter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A reboot of the 80's Microwriter accessible chord keyboard done using an Arduino.", NewPluralDecades(), "A reboot of the '80s Microwriter accessible chord keyboard done using an Arduino.")
}

func TestPluralDecadesTwoDigits_Fix80sNeonTheme(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "A flat, 80's neon inspired theme for JupyterLab.", NewPluralDecades(), "A flat, '80s neon inspired theme for JupyterLab.")
}

func TestPluralDecadesTwoDigits_Fix80sNeonThemeColors(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Cool UI Theme for Atom based on 80's neon colors with big tabs for easy files Switch.", NewPluralDecades(), "Cool UI Theme for Atom based on '80s neon colors with big tabs for easy files Switch.")
}

func TestPluralDecadesTwoDigits_Fix80sSynthwaveTheme(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "An clean 80's synthwave / outrun inspired theme for Vim.", NewPluralDecades(), "An clean '80s synthwave / outrun inspired theme for Vim.")
}

func TestPluralDecadesTwoDigits_Fix80s3dEra(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Experimenting with writing 80's era 3D code but in Javascript and with HTML5 Canvas acting as display buffer.", NewPluralDecades(), "Experimenting with writing '80s era 3D code but in Javascript and with HTML5 Canvas acting as display buffer.")
}

func TestPluralDecadesTwoDigits_Fix80sTheme80sAesthetics(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Vibrant 80's Klipper Mainsail Theme, based around 80's Dark Neon Aesthetics.", NewPluralDecades(), "Vibrant '80s Klipper Mainsail Theme, based around '80s Dark Neon Aesthetics.")
}

func TestPluralDecadesTwoDigits_Fix80sDarkRetroTheme(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "80's dark retro theme for VS Code and Sublime Text", NewPluralDecades(), "'80s dark retro theme for VS Code and Sublime Text")
}

func TestPluralDecadesTwoDigits_Fix80sChorusEffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "An 80's style chorus effect for your KORG 'logue synthesizers - hammondeggs/hera.", NewPluralDecades(), "An '80s style chorus effect for your KORG 'logue synthesizers - hammondeggs/hera.")
}

func TestPluralDecadesTwoDigits_DontFlagChrome80s(t *testing.T) {
	t.Skip("Does this mean a Chrome browser version? If so what's the possessive for?")
	linttest.AssertNoLints(t, "Ready for Chrome 80's [Cookies default to SameSite=Lax] ?", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_Fix80sStyle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Made your RStudio 80's style only after the sun goes down.", NewPluralDecades(), "Made your RStudio '80s style only after the sun goes down.")
}

func TestPluralDecadesTwoDigits_FixEarly80s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hardware and a game for the early 80's Z8671 MCU with built-in BASIC", NewPluralDecades(), "Hardware and a game for the early '80s Z8671 MCU with built-in BASIC")
}

func TestPluralDecadesTwoDigits_FixModest80sFighterGame(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "An attempt by the MEGA65 community to write a modest 80's fighter game with BASIC 65 + Eleven", NewPluralDecades(), "An attempt by the MEGA65 community to write a modest '80s fighter game with BASIC 65 + Eleven")
}

func TestPluralDecadesTwoDigits_FixStraightFrom80s(t *testing.T) {
	t.Skip("probably nonnative Engish for 'from the 80s'")
	linttest.AssertSuggestionResult(t, "Straight from 80's", NewPluralDecades(), "Straight from the '80s")
}

func TestPluralDecadesTwoDigits_FixThe90sWere(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Generate animated vector graphics for old-school 90's demos, like ST_NICCC", NewPluralDecades(), "Generate animated vector graphics for old-school '90s demos, like ST_NICCC")
}

func TestPluralDecadesTwoDigits_FixLate90s(t *testing.T) {
	t.Skip("we detect `your 90's` as an age range, not a decade")
	linttest.AssertSuggestionResult(t, "gmdrec is a USB interface between your late 90's Sony portable MiniDisc recorder and your PC.", NewPluralDecades(), "gmdrec is a USB interface between your late '90s Sony portable MiniDisc recorder and your PC.")
}

func TestPluralDecadesTwoDigits_FixThe90sNpsg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A modern vision on the 90's game Log!cal.", NewPluralDecades(), "A modern vision on the '90s game Log!cal.")
}

func TestPluralDecadesTwoDigits_FixFromThe90s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Digital Sound and Music Interface (from the 90's).", NewPluralDecades(), "Digital Sound and Music Interface (from the '90s).")
}

func TestPluralDecadesTwoDigits_FixTheLate90s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A modified CircleMUD that ran in the late 90's.", NewPluralDecades(), "A modified CircleMUD that ran in the late '90s.")
}

func TestPluralDecadesTwoDigits_FixRad90sWebsite(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "This is our rad 90's website.", NewPluralDecades(), "This is our rad '90s website.")
}

func TestPluralDecadesTwoDigits_FixMixed80sAnd90s(t *testing.T) {
	t.Skip("mixed 80 with no 's next to 90's with 's might be too oddball")
	linttest.AssertSuggestionResult(t, "\"ワープロ明朝\" is a font that reproduced the smoothing algorithm used in the 80-90's Japanese word processors.", NewPluralDecades(), "\"ワープロ明朝\" is a font that reproduced the smoothing algorithm used in the 80s-'90s Japanese word processors.")
}

func TestPluralDecadesTwoDigits_DontFlagAll90Degrees(t *testing.T) {
	t.Skip("90 degrees")
	linttest.AssertNoLints(t, "get_map(\"Slope Degrees\") returns all 90's unless projected crs is specified", NewPluralDecades())
}

func TestPluralDecadesTwoDigits_FixA90sWorkstation(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "a 90's workstation; now likely too small", NewPluralDecades(), "a '90s workstation; now likely too small")
}

func TestPluralDecadesTwoDigits_FixDomainsInThe90s(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Whois for gems, because gem names are like domains in the 90's", NewPluralDecades(), "Whois for gems, because gem names are like domains in the '90s")
}

func TestPluralDecadesTwoDigits_FixTheClassic90sGame(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "Dragon Court, the classic 90's game by Fred Haslam", NewPluralDecades(), "Dragon Court, the classic '90s game by Fred Haslam")
}

func TestPluralDecadesTwoDigits_FixMultipleAges(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "It generates 100,000 random \"people\" and randomly assigns them as being in their 20's, 30's, 40's, 50's, 60's, or 70's.", NewPluralDecades(), "It generates 100,000 random \"people\" and randomly assigns them as being in their 20s, 30s, 40s, 50s, 60s, or 70s.")
}

func TestPluralDecadesTwoDigits_FixMissingThe(t *testing.T) {
	t.Skip("not sure if we should support missing 'the', especially when there's two decades")
	linttest.AssertSuggestionResult(t, "A thoughtful full-stack reimplementation of gaming in 80's and 90's.", NewPluralDecades(), "A thoughtful full-stack reimplementation of gaming in the '80s and '90s.")
}

func TestPluralDecadesTwoDigits_FixMy20sAnd30s(t *testing.T) {
	t.Skip("wip")
	linttest.AssertSuggestionResult(t, "I spend my 20's and 30's as an officer with Royal Caribbean Cruises", NewPluralDecades(), "I spend my 20s and 30s as an officer with Royal Caribbean Cruises")
}
