package rules

import (
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPhraseSetCorrectionsManyToManyTests_CorrectAwaitsFor(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Headless mode awaits for requested user feedback without showing any text for what that feedback should be", phraseSetCorrectionsTestLinter, []string{
		"Headless mode awaits requested user feedback without showing any text for what that feedback should be",
		"Headless mode waits for requested user feedback without showing any text for what that feedback should be",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectAwaitingFor(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "gpg import fails awaiting for prompt answer", phraseSetCorrectionsTestLinter, []string{
		"gpg import fails waiting for prompt answer",
		"gpg import fails awaiting prompt answer",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectAwaitFor(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I still await for a college course on \"Followership 101\"", phraseSetCorrectionsTestLinter, []string{
		"I still wait for a college course on \"Followership 101\"",
		"I still await a college course on \"Followership 101\"",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectAwaitedFor(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I have long awaited for the rise of the Dagoat agenda, and it is glorious.", phraseSetCorrectionsTestLinter, []string{
		"I have long awaited the rise of the Dagoat agenda, and it is glorious.",
		"I have long waited for the rise of the Dagoat agenda, and it is glorious.",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_BackwardSpaceEngineer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The idea would be to backward engineer such \"creation process\" to the extent given by tools and abilities.", phraseSetCorrectionsTestLinter, "The idea would be to reverse engineer such \"creation process\" to the extent given by tools and abilities.")
}

func TestPhraseSetCorrectionsManyToManyTests_BackwardHyphenEngineer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Use Assignment 5 screenshots and R Manual to backward-engineer Motor Trend Car Road Tests data", phraseSetCorrectionsTestLinter, "Use Assignment 5 screenshots and R Manual to reverse-engineer Motor Trend Car Road Tests data")
}

func TestPhraseSetCorrectionsManyToManyTests_BackwardHyphenEngineering(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For the backward-engineering to work, I realize that the power spectra we have saved were done using these super high-accuracy settings", phraseSetCorrectionsTestLinter, "For the reverse-engineering to work, I realize that the power spectra we have saved were done using these super high-accuracy settings")
}

func TestPhraseSetCorrectionsManyToManyTests_BackwardSpaceEngineering(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Note though, that CS is \"forward\" engineering, while Brain Science is backward engineering.", phraseSetCorrectionsTestLinter, "Note though, that CS is \"forward\" engineering, while Brain Science is reverse engineering.")
}

func TestPhraseSetCorrectionsManyToManyTests_BackwardsHyphenEngineer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "After leaving this talk, you'll be able to backwards-engineer these 'internal' APIs and replicate any trivial flow", phraseSetCorrectionsTestLinter, "After leaving this talk, you'll be able to reverse-engineer these 'internal' APIs and replicate any trivial flow")
}

func TestPhraseSetCorrectionsManyToManyTests_BackwardsSpaceEngineer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "do this and you can backwards engineer what you should be getting", phraseSetCorrectionsTestLinter, "do this and you can reverse engineer what you should be getting")
}

func TestPhraseSetCorrectionsManyToManyTests_BackwardsSpaceEngineering(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This chapter gives an overview the first assessment, the backwards engineering report along with the files you will need and some helpful hints.", phraseSetCorrectionsTestLinter, "This chapter gives an overview the first assessment, the reverse engineering report along with the files you will need and some helpful hints.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectBackhandCompliment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What's the Most insulting /backhand compliment you have ever received?", phraseSetCorrectionsTestLinter, "What's the Most insulting /backhanded compliment you have ever received?")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectBackHandComplimentSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thankfully, I'm a little young to receive that back hand compliment", phraseSetCorrectionsTestLinter, "Thankfully, I'm a little young to receive that backhanded compliment")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectBackHandComplimentHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They \"have it all\", but still need to back-hand compliment, condescend, \"politely\" hurl passive-aggressive compliments/insults", phraseSetCorrectionsTestLinter, "They \"have it all\", but still need to backhanded compliment, condescend, \"politely\" hurl passive-aggressive compliments/insults")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectBackhandCompliments(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If backhand compliments or flattery are frequent and come with ulterior motives, it's a red flag.", phraseSetCorrectionsTestLinter, "If backhanded compliments or flattery are frequent and come with ulterior motives, it's a red flag.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectBackHandCompliments(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am laughing so hard watching the \"commercial\" with her back hand compliments.", phraseSetCorrectionsTestLinter, "I am laughing so hard watching the \"commercial\" with her backhanded compliments.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectBackHandComplimentsCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "JUST SAY YOU DON'T LIKE THEM STOP GIVING FAKE BACK HAND COMPLIMENTS.", phraseSetCorrectionsTestLinter, "JUST SAY YOU DON'T LIKE THEM STOP GIVING FAKE BACKHANDED COMPLIMENTS.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectBackHandComplimentsHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no praises, just tons of unnecessary back-hand compliments and lack of support", phraseSetCorrectionsTestLinter, "no praises, just tons of unnecessary backhanded compliments and lack of support")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBainOfMyExistance(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is the bain of my existance, please help.", phraseSetCorrectionsTestLinter, "This is the bane of my existence, please help.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBainOfOur(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They are the bain of our existence on Apple hardware", phraseSetCorrectionsTestLinter, "They are the bane of our existence on Apple hardware")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBainOfTheir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "individual people in Apple might decide port scanning is the bain of their existence and send something", phraseSetCorrectionsTestLinter, "individual people in Apple might decide port scanning is the bane of their existence and send something")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBaneMarie(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The bane-marie is a favorite metaphor in my life for adding 'heat' (usually stress) to any idea, process, or person", phraseSetCorrectionsTestLinter, "The bain-marie is a favorite metaphor in my life for adding 'heat' (usually stress) to any idea, process, or person")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBaneMaries(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "They have bane Maries which is just a double broiler (water heated high under pans).", phraseSetCorrectionsTestLinter, []string{

		"They have bain Maries which is just a double broiler (water heated high under pans).",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_SingularTowards(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the platform's focus on multimedia projects and VideoLAN's long history of commitment towards free and open multimedia", phraseSetCorrectionsTestLinter, "the platform's focus on multimedia projects and VideoLAN's long history of commitment to free and open multimedia")
}

func TestPhraseSetCorrectionsManyToManyTests_PluralTowards(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the signer may express multiple commitments towards the data objects", phraseSetCorrectionsTestLinter, "the signer may express multiple commitments to the data objects")
}

func TestPhraseSetCorrectionsManyToManyTests_SingularToward(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This document outlines the current level of commitment toward Linux distributions and packaging formats.", phraseSetCorrectionsTestLinter, "This document outlines the current level of commitment to Linux distributions and packaging formats.")
}

func TestPhraseSetCorrectionsManyToManyTests_PluralToward(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... and are expected to inform parties in updating their commitments toward the Paris Agreement", phraseSetCorrectionsTestLinter, "... and are expected to inform parties in updating their commitments to the Paris Agreement")
}

func TestPhraseSetCorrectionsManyToManyTests_Copywritten(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Including digital copies of copywritten artwork with the project isn't advised.", phraseSetCorrectionsTestLinter, "Including digital copies of copyrighted artwork with the project isn't advised.")
}

func TestPhraseSetCorrectionsManyToManyTests_Copywrites(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Code is 99% copy/pasted from OpenSSH with an attempt to retain all copywrites", phraseSetCorrectionsTestLinter, "Code is 99% copy/pasted from OpenSSH with an attempt to retain all copyrights")
}

func TestPhraseSetCorrectionsManyToManyTests_Copywrited(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Proprietary copywrited code", phraseSetCorrectionsTestLinter, "Proprietary copyrighted code")
}

func TestPhraseSetCorrectionsManyToManyTests_CopywritedAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "URLS MAY CONTAIN COPYWRITED MATERIAL", phraseSetCorrectionsTestLinter, "URLS MAY CONTAIN COPYRIGHTED MATERIAL")
}

func TestPhraseSetCorrectionsManyToManyTests_Copywrote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How do you find out if someone copywrote a movie", phraseSetCorrectionsTestLinter, "How do you find out if someone copyrighted a movie")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsDateBackFrom(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "There are too many open issues that date back from 4 years ago.", phraseSetCorrectionsTestLinter, []string{
		"There are too many open issues that date from 4 years ago.",
		"There are too many open issues that date back to 4 years ago.",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsDatesBackFrom(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "This code dates back from 2014.", phraseSetCorrectionsTestLinter, []string{
		"This code dates from 2014.",
		"This code dates back to 2014.",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_AllowsDateBackTo(t *testing.T) {
	linttest.AssertNoLints(t, "These scripts date back to when Perl was popular.", phraseSetCorrectionsTestLinter)
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectDoubleEdgeHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I thought the global defaultTranslationValues was potentially a double-edge sword as it also obfuscates the full set of values", phraseSetCorrectionsTestLinter, "I thought the global defaultTranslationValues was potentially a double-edged sword as it also obfuscates the full set of values")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectDoubleEdgeSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It becomes a double edge sword when it should not be used in cases like this.", phraseSetCorrectionsTestLinter, "It becomes a double-edged sword when it should not be used in cases like this.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectDoubleEdgeSpacePlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wake locks are really double edge swords.", phraseSetCorrectionsTestLinter, "Wake locks are really double-edged swords.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectDoubleEdgedSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Use case. currently OPTIMIZE is a double edged sword and potentially a very dangerous tool to use.", phraseSetCorrectionsTestLinter, "Use case. currently OPTIMIZE is a double-edged sword and potentially a very dangerous tool to use.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectDoubleEdgedSpacePlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Change: Ambushers and Crusaders now protect their targets too, making them double edged swords", phraseSetCorrectionsTestLinter, "Change: Ambushers and Crusaders now protect their targets too, making them double-edged swords")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsAllocs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "cmd/compile: avoid allocs by better tracking of literals for interface conversions and make", phraseSetCorrectionsTestLinter, "cmd/compile: avoid allocations by better tracking of literals for interface conversions and make")
}

func TestPhraseSetCorrectionsManyToManyTests_ExpandAlloc(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Used to find system libraries that alloc RWX regions on load.", phraseSetCorrectionsTestLinter, "Used to find system libraries that allocate RWX regions on load.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsGovtNoDot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Separation between privately issued credentials vs govt issued identity credentials", phraseSetCorrectionsTestLinter, "Separation between privately issued credentials vs government issued identity credentials")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsGovtDo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Demystifying public comments on govt. regulations.", phraseSetCorrectionsTestLinter, "Demystifying public comments on government regulations.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsGovts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Those 'elite' economists have been advising govts for years.", phraseSetCorrectionsTestLinter, "Those 'elite' economists have been advising governments for years.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectExPatHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It seems ex-pat means the person will be in a foreign country temporarily", phraseSetCorrectionsTestLinter, "It seems expat means the person will be in a foreign country temporarily")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectExPatsHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So, it might be correct to call most Brits ex-pats.", phraseSetCorrectionsTestLinter, "So, it might be correct to call most Brits expats.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectExPatSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For me, the term ex pat embodies the exquisite hypocrisy of certain people feeling entitled", phraseSetCorrectionsTestLinter, "For me, the term expat embodies the exquisite hypocrisy of certain people feeling entitled")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectExPatsSpace(t *testing.T) {
	t.Skip("replace_with_match_case results in ExPats")
	linttest.AssertSuggestionResult(t, "Why are Brits who emigrate \"Ex Pats\" but people who come here \"immigrants\"?", phraseSetCorrectionsTestLinter, "Why are Brits who emigrate \"Expats\" but people who come here \"immigrants\"?")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectExpatriot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Another expatriot of the era, James Joyce, also followed Papa's writing and drinking schedule.", phraseSetCorrectionsTestLinter, "Another expatriate of the era, James Joyce, also followed Papa's writing and drinking schedule.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectExpatriots(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Expatriots, upon discovering the delightful nuances of Dutch pronunciation, often find themselves in stitches.", phraseSetCorrectionsTestLinter, "Expatriates, upon discovering the delightful nuances of Dutch pronunciation, often find themselves in stitches.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectExPatriotHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Then I added we should all be using the word 移民 immigrant, not ex-patriot, not 外国人 gaikokujin, and definitely not 外人 gaijin", phraseSetCorrectionsTestLinter, "Then I added we should all be using the word 移民 immigrant, not expatriate, not 外国人 gaikokujin, and definitely not 外人 gaijin")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectExPatriotsHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ex-patriots who move to Hong Kong to seek greener pastures and to experience a new culture seem to bring their own cultural baggage with them.", phraseSetCorrectionsTestLinter, "Expatriates who move to Hong Kong to seek greener pastures and to experience a new culture seem to bring their own cultural baggage with them.")
}

func TestPhraseSetCorrectionsManyToManyTests_GetRidOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please bump axios version to get rid off npm warning #624", phraseSetCorrectionsTestLinter, "Please bump axios version to get rid of npm warning #624")
}

func TestPhraseSetCorrectionsManyToManyTests_GetsRidOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Adding at as a runtime dependency gets rid off that error", phraseSetCorrectionsTestLinter, "Adding at as a runtime dependency gets rid of that error")
}

func TestPhraseSetCorrectionsManyToManyTests_GettingRidOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "getting rid off of all the complexity of the different accesses method of API service providers", phraseSetCorrectionsTestLinter, "getting rid of of all the complexity of the different accesses method of API service providers")
}

func TestPhraseSetCorrectionsManyToManyTests_GotRidOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For now we got rid off circular deps in model tree structure and it's API.", phraseSetCorrectionsTestLinter, "For now we got rid of circular dependencies in model tree structure and it's API.")
}

func TestPhraseSetCorrectionsManyToManyTests_GottenRidOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The baX variable thingy I have gotten rid off, that was due to a bad character in the encryption key.", phraseSetCorrectionsTestLinter, "The baX variable thingy I have gotten rid of, that was due to a bad character in the encryption key.")
}

func TestPhraseSetCorrectionsManyToManyTests_GetRideOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Get ride of \"WARNING Deprecated: markdown_github. Use gfm\"", phraseSetCorrectionsTestLinter, "Get rid of \"WARNING Deprecated: markdown_github. Use gfm\"")
}

func TestPhraseSetCorrectionsManyToManyTests_GetRideOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This exact hack was what I trying to get ride off. ", phraseSetCorrectionsTestLinter, "This exact hack was what I trying to get rid of. ")
}

func TestPhraseSetCorrectionsManyToManyTests_GettingRideOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you have any idea how to fix this without getting ride of bootstrap I would be thankfull.", phraseSetCorrectionsTestLinter, "If you have any idea how to fix this without getting rid of bootstrap I would be thankfull.")
}

func TestPhraseSetCorrectionsManyToManyTests_GetsRideOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, ".. gets ride of a central back-end/server and eliminates all the risks associated to it.", phraseSetCorrectionsTestLinter, ".. gets rid of a central back-end/server and eliminates all the risks associated to it.")
}

func TestPhraseSetCorrectionsManyToManyTests_GottenRideOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have gotten ride of the react-table and everything works just fine.", phraseSetCorrectionsTestLinter, "I have gotten rid of the react-table and everything works just fine.")
}

func TestPhraseSetCorrectionsManyToManyTests_GotRideOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I had to adjust the labels on the free version because you guys got ride of ...", phraseSetCorrectionsTestLinter, "I had to adjust the labels on the free version because you guys got rid of ...")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHallucinisation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "candybar left over from halloween [mmm moldy hallucinisation goodness! >_>]", phraseSetCorrectionsTestLinter, "candybar left over from halloween [mmm moldy hallucination goodness! >_>]")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHallucinisations(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I havent really experienced hallucinisations or delusions, but this interpertation of movie is extremely intriguing", phraseSetCorrectionsTestLinter, "I havent really experienced hallucinations or delusions, but this interpertation of movie is extremely intriguing")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHallucinization(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The hallucinization machine makers will be long gone to their retirement homes on private islands", phraseSetCorrectionsTestLinter, "The hallucination machine makers will be long gone to their retirement homes on private islands")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHallucinizations(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He was treated for schizophrenia so I think the meds helped him with the voices and hallucinizations.", phraseSetCorrectionsTestLinter, "He was treated for schizophrenia so I think the meds helped him with the voices and hallucinations.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHiHyphenJacked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Empty message in hi-jacked POST from javascript to C# MVC", phraseSetCorrectionsTestLinter, "Empty message in hijacked POST from javascript to C# MVC")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHiHyphenJackers(t *testing.T) {
	linttest.AssertSuggestionResult(t, "hi-jackers, including one with no experience at all, survived jumps", phraseSetCorrectionsTestLinter, "hijackers, including one with no experience at all, survived jumps")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHiHyphenJacking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "should be thrown instead of hi-jacking the Error class to dynamically add a property to the object", phraseSetCorrectionsTestLinter, "should be thrown instead of hijacking the Error class to dynamically add a property to the object")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHiHyphenJacks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Instruments' UI Recorder hi-jacks my entire system", phraseSetCorrectionsTestLinter, "Instruments' UI Recorder hijacks my entire system")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHighSpaceJack(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ISPs that high jack the nxdomain result to send you sponsored results of vaguely similar sounding websites", phraseSetCorrectionsTestLinter, "ISPs that hijack the nxdomain result to send you sponsored results of vaguely similar sounding websites")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHighHyphenJack(t *testing.T) {
	linttest.AssertSuggestionResult(t, "because now we no longer high-jack the API calls", phraseSetCorrectionsTestLinter, "because now we no longer hijack the API calls")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHighHyphenJacked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "not possible for the moment because the click event is high-jacked", phraseSetCorrectionsTestLinter, "not possible for the moment because the click event is hijacked")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHighHyphenJacker(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Driving a #Polo will get you treated like some high-jacker or car thief on the road", phraseSetCorrectionsTestLinter, "Driving a #Polo will get you treated like some hijacker or car thief on the road")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHighSpaceJackers(t *testing.T) {
	linttest.AssertSuggestionResult(t, "America and the high jackers are both referring to the same act of terror.", phraseSetCorrectionsTestLinter, "America and the hijackers are both referring to the same act of terror.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHighSpaceJacking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Security issue , Session high jacking prevention", phraseSetCorrectionsTestLinter, "Security issue , Session hijacking prevention")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHighHyphenJacks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Kotlin Compiler Plugin which high-jacks Kotlin assert function calls", phraseSetCorrectionsTestLinter, "Kotlin Compiler Plugin which hijacks Kotlin assert function calls")
}

func TestPhraseSetCorrectionsManyToManyTests_FixHighSpaceJacks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This high jacks the issue somewhat but, the Computation Scheduler only runs as many concurrent threads as there are cores", phraseSetCorrectionsTestLinter, "This hijacks the issue somewhat but, the Computation Scheduler only runs as many concurrent threads as there are cores")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHolyWar(t *testing.T) {
	t.Skip("Known failure due to replace_with_match_case working by character index")
	linttest.AssertSuggestionResult(t, "I know it is Holly War about idempotent in HTTP and DELETE", phraseSetCorrectionsTestLinter, "I know it is Holy War about idempotent in HTTP and DELETE")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHollyWars(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Anyway I'm not starting some holly wars about this point.", phraseSetCorrectionsTestLinter, "Anyway I'm not starting some holy wars about this point.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowItLooksLike1(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And here is how it looks like: As you can see, there is no real difference in the diagram itself.", phraseSetCorrectionsTestLinter, "And here is how it looks: As you can see, there is no real difference in the diagram itself.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowItLooksLike2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is how it looks like when run from Windows PowerShell or Cmd: image.", phraseSetCorrectionsTestLinter, "This is what it looks like when run from Windows PowerShell or Cmd: image.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowTheyLookLike1(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a sample project illustrating a demo of how to use the new Material 3 components and how they look like.", phraseSetCorrectionsTestLinter, "This is a sample project illustrating a demo of how to use the new Material 3 components and how they look.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowTheyLookLike2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So for now I'll just leave this issue here of how they look like in the XLSX", phraseSetCorrectionsTestLinter, "So for now I'll just leave this issue here of what they look like in the XLSX")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowTheyLooksLike1(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here I demonstrate how disney works and how they looks like Don't miss to give me a star.", phraseSetCorrectionsTestLinter, "Here I demonstrate how disney works and how they look Don't miss to give me a star.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowTheyLooksLike2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can check how they looks like on Android app by this command:", phraseSetCorrectionsTestLinter, "You can check what they look like on Android app by this command:")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowSheLooksLike1(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You all know how she looks like.", phraseSetCorrectionsTestLinter, "You all know how she looks.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowHeLooksLike2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here's how he looks like, when he's supposed to just look like his old fatui design.", phraseSetCorrectionsTestLinter, "Here's what he looks like, when he's supposed to just look like his old fatui design.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowItLookLike1(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And I don't mind how it look like, language code subpath or the last subpath as below.", phraseSetCorrectionsTestLinter, "And I don't mind how it looks, language code subpath or the last subpath as below.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowItLookLike2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here is how it look like in your browser:", phraseSetCorrectionsTestLinter, "Here is what it looks like in your browser:")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectHowItLooksLikeWithApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the picture we can see how It look's like on worker desktop.", phraseSetCorrectionsTestLinter, "In the picture we can see how It looks on worker desktop.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsInRetaliationToToFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Damage caused in retaliation to another attack by the Thorns enchantment.", phraseSetCorrectionsTestLinter, "Damage caused in retaliation for another attack by the Thorns enchantment.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsInRetaliationToToInResponseTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In retaliation to disagreeing with legal naming issues, a crucial (albeit rather small) section of code was removed from the NPM database", phraseSetCorrectionsTestLinter, "In response to disagreeing with legal naming issues, a crucial (albeit rather small) section of code was removed from the NPM database")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLevelOfDetailsSingularContrived(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The model has a high level of details.", phraseSetCorrectionsTestLinter, "The model has a high level of detail.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLevelOfDetailsSingularRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How to implement a level of details visualizer for 3D meshes?", phraseSetCorrectionsTestLinter, "How to implement a level of detail visualizer for 3D meshes?")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLevelOfDetailsPluralRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "LOD's (Level of details) are a set of lower models used for the purpose of optimisation", phraseSetCorrectionsTestLinter, "LOD's (Levels of detail) are a set of lower models used for the purpose of optimisation")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLevelsOfDetailsRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The file completion uses two levels of details to optimize performance.", phraseSetCorrectionsTestLinter, "The file completion uses two levels of detail to optimize performance.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLookALike(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Define the look-a-like of the cursor/mouse pointer:", phraseSetCorrectionsTestLinter, []string{
		"Define the lookalike of the cursor/mouse pointer:",
		"Define the look-alike of the cursor/mouse pointer:",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLookALikes(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Attempt at using AWS facial recognition to find look-a-likes in the Rijksmuseum's art collection.", phraseSetCorrectionsTestLinter, []string{
		"Attempt at using AWS facial recognition to find lookalikes in the Rijksmuseum's art collection.",
		"Attempt at using AWS facial recognition to find look-alikes in the Rijksmuseum's art collection.",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_FixLookForwardFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I will mark this issue as an enhancement and will look forward for enrolling it.", phraseSetCorrectionsTestLinter, "I will mark this issue as an enhancement and will look forward to enrolling it.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixLookedForwardFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Looked forward for standalone components so much, please fix this.", phraseSetCorrectionsTestLinter, "Looked forward to standalone components so much, please fix this.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixLookingForwardFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Looking forward for Typed version of this stack navigation", phraseSetCorrectionsTestLinter, "Looking forward to Typed version of this stack navigation")
}

func TestPhraseSetCorrectionsManyToManyTests_FixLooksForwardFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please take this words as from one of your fans who looks forward for a great and interesting project :)", phraseSetCorrectionsTestLinter, "Please take this words as from one of your fans who looks forward to a great and interesting project :)")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLookForwardsTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I look forwards to getting to make use of this in 2040!", phraseSetCorrectionsTestLinter, "I look forward to getting to make use of this in 2040!")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLookingForwardsTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am looking forwards to digging in to this a bit more and figuring out how this compares to Istio.", phraseSetCorrectionsTestLinter, "I am looking forward to digging in to this a bit more and figuring out how this compares to Istio.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLooksForwardsTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "in a recession, the optimist looks forwards to the next bull run", phraseSetCorrectionsTestLinter, "in a recession, the optimist looks forward to the next bull run")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsLookedForwardsTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I looked forwards to reading his updates and it looks like he stopped a while ago…", phraseSetCorrectionsTestLinter, "I looked forward to reading his updates and it looks like he stopped a while ago…")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsMakeItSeems(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but put it into unlisted list may make it seems like listed for GitHub", phraseSetCorrectionsTestLinter, "but put it into unlisted list may make it seem like listed for GitHub")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsMadeItSeems(t *testing.T) {
	linttest.AssertSuggestionResult(t, "previous explanations made it seems like it would be n", phraseSetCorrectionsTestLinter, "previous explanations made it seem like it would be n")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsMakesItSeems(t *testing.T) {
	linttest.AssertSuggestionResult(t, "bundle gives an error that makes it seems like esbuild is trying to use lib/index.js from main", phraseSetCorrectionsTestLinter, "bundle gives an error that makes it seem like esbuild is trying to use lib/index.js from main")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsMakingItSeems(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is it possible to teach the concept of assignment/reassignment at the very beginner stage instead of making it seems like constants?", phraseSetCorrectionsTestLinter, "Is it possible to teach the concept of assignment/reassignment at the very beginner stage instead of making it seem like constants?")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsMadeItSeemed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The path made it seemed a bit \"internal\".", phraseSetCorrectionsTestLinter, "The path made it seem a bit \"internal\".")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsMonumentous(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think that would be a monumentous step in the right direction, and would DEFINATLY turn heads in not just the music industry, but every ...", phraseSetCorrectionsTestLinter, "I think that would be a momentous step in the right direction, and would DEFINATLY turn heads in not just the music industry, but every ...")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectsMonumentously(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the most impressive thing out of all of this is that GitHub created such a monumentously good name", phraseSetCorrectionsTestLinter, "the most impressive thing out of all of this is that GitHub created such a monumentally good name")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectNerveWreckSpaceTitleCase(t *testing.T) {
	t.Skip("Harper matches case by letter index as 'How Not to Be a Complete NervoUs wreck in an Interview'")
	linttest.AssertSuggestionResult(t, "How Not to Be a Complete Nerve Wreck in an Interview", phraseSetCorrectionsTestLinter, "How Not to Be a Complete Nervous Wreck in an Interview")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectNerveWreckSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The nerve wreck you are makes you seem anxious and agitated so your employer will believe the complaints.", phraseSetCorrectionsTestLinter, "The nervous wreck you are makes you seem anxious and agitated so your employer will believe the complaints.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectNerveWreckHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the child receives little education and grows up to be a nerve-wreck", phraseSetCorrectionsTestLinter, "the child receives little education and grows up to be a nervous wreck")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectNerveWreckHyphenPlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This helps us not to become nerve wrecks while looking at the side mirrors", phraseSetCorrectionsTestLinter, "This helps us not to become nervous wrecks while looking at the side mirrors")
}

func TestPhraseSetCorrectionsManyToManyTests_DontCorrectItWasANerveWreck(t *testing.T) {
	t.Skip("We can't detect when the altered form is used for an event rather than a person.")
	linttest.AssertNoLints(t, "It was a nerve-wreck, but I was also excited to see what would happen next.", phraseSetCorrectionsTestLinter)
}

func TestPhraseSetCorrectionsManyToManyTests_DontCorrectSoMuchNerveWreck(t *testing.T) {
	t.Skip("We can't detect when the altered form is used for an event rather than a person.")
	linttest.AssertNoLints(t, "So much nerve wreck for such a simple game ...", phraseSetCorrectionsTestLinter)
}

func TestPhraseSetCorrectionsManyToManyTests_FixNoOnlyAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No only are tests run on my pipeline but once successful, my app is deployed differently", phraseSetCorrectionsTestLinter, "Not only are tests run on my pipeline but once successful, my app is deployed differently")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNoOnlyIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No only is it simple, it's efficient!", phraseSetCorrectionsTestLinter, "Not only is it simple, it's efficient!")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNoOnlyWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No only was he happily creating shapes, but he was actively using distances and angles to do so.", phraseSetCorrectionsTestLinter, "Not only was he happily creating shapes, but he was actively using distances and angles to do so.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNoOnlyWere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No only were there UI inconsistencies, but Safari lags behind chrome with things like the Popover API", phraseSetCorrectionsTestLinter, "Not only were there UI inconsistencies, but Safari lags behind chrome with things like the Popover API")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowADaysSpaces(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Now a days, movie recommendation systems are well developed and are user focused.", phraseSetCorrectionsTestLinter, "Nowadays, movie recommendation systems are well developed and are user focused.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowADaysApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Now a day's recognizing the activity from the surveillance video is a challenging task.", phraseSetCorrectionsTestLinter, "Nowadays recognizing the activity from the surveillance video is a challenging task.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowADaysHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Recommendation engines are now a one of the most common Machine Learning project that can be seen now-a-days.", phraseSetCorrectionsTestLinter, "Recommendation engines are now a one of the most common Machine Learning project that can be seen nowadays.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowADay(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Now a day a calendar is a daily essential things.", phraseSetCorrectionsTestLinter, "Nowadays a calendar is a daily essential things.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowADayHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Now-a-day, lots of people prefer ordering food online to save their time and effort.", phraseSetCorrectionsTestLinter, "Nowadays, lots of people prefer ordering food online to save their time and effort.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowAdaysHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "@andyp1per knows most about those Python scripts now-adays.", phraseSetCorrectionsTestLinter, "@andyp1per knows most about those Python scripts nowadays.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowAdaysSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Coding is one of my fav thing to do now adays.!", phraseSetCorrectionsTestLinter, "Coding is one of my fav thing to do nowadays.!")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowaday(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nowaday, I have to capitalize the first letter of the name gets from @babel/types.", phraseSetCorrectionsTestLinter, "nowadays, I have to capitalize the first letter of the name gets from @babel/types.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowAdaysApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I believe CSS clamp has great browser support now aday's as well.", phraseSetCorrectionsTestLinter, "I believe CSS clamp has great browser support nowadays as well.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowaDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But discord would be great cause discord is universally used for all games and companies and schools nowa days.", phraseSetCorrectionsTestLinter, "But discord would be great cause discord is universally used for all games and companies and schools nowadays.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixNowAday(t *testing.T) {
	linttest.AssertSuggestionResult(t, "OF all Occupations that now aday is used,I would not be a butcher", phraseSetCorrectionsTestLinter, "OF all Occupations that nowadays is used,I would not be a butcher")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectPayed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He payed the bill yesterday.", phraseSetCorrectionsTestLinter, "He paid the bill yesterday.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectOverpayed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He overpayed in part to have the specification met.", phraseSetCorrectionsTestLinter, "He overpaid in part to have the specification met.")
}

func TestPhraseSetCorrectionsManyToManyTests_PlayAFactor(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I thought scaling might play a factor in this so I made sure it was 100% on all three desktops.", phraseSetCorrectionsTestLinter, []string{
		"I thought scaling might play a part in this so I made sure it was 100% on all three desktops.",
		"I thought scaling might be a factor in this so I made sure it was 100% on all three desktops.",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_PlayedAFactorSg(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I want you to look past them, because none of them played a factor in composing a dream team.", phraseSetCorrectionsTestLinter, []string{
		"I want you to look past them, because none of them played a part in composing a dream team.",
		"I want you to look past them, because none of them was a factor in composing a dream team.",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_PlayedAFactorPl(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "we have no idea why they do what they do and what past external influences played factors in those decisions", phraseSetCorrectionsTestLinter, []string{
		"we have no idea why they do what they do and what past external influences played roles in those decisions",
		"we have no idea why they do what they do and what past external influences were a factor in those decisions",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_PlayingAFactor(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I think my development 'forceIP' was playing a factor here.", phraseSetCorrectionsTestLinter, []string{
		"I think my development 'forceIP' was playing a part here.",
		"I think my development 'forceIP' was a factor here.",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_PlaysAFactor(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "The amount of time since the change certainly plays a factor since we upgraded to eslint v6 3 years ago.", phraseSetCorrectionsTestLinter, []string{
		"The amount of time since the change certainly plays a part since we upgraded to eslint v6 3 years ago.",
		"The amount of time since the change certainly is a factor since we upgraded to eslint v6 3 years ago.",
	}, []string{})
}

func TestPhraseSetCorrectionsManyToManyTests_DetectRiseTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That would rise the question how to deal with syntax errors etc.", phraseSetCorrectionsTestLinter, "That would raise the question how to deal with syntax errors etc.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectAriseTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As e.g. UTC+1, might arise the question whether it includes summer and winter time", phraseSetCorrectionsTestLinter, "As e.g. UTC+1, might raise the question whether it includes summer and winter time")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectRisesTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However, this rises the question as to whether this test is conceptually sound.", phraseSetCorrectionsTestLinter, "However, this raises the question as to whether this test is conceptually sound.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectArisesTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And it arises the question, why?", phraseSetCorrectionsTestLinter, "And it raises the question, why?")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectRisingTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "as soon as a infoHash query is performed, a Torrent file is retried, rising the question of:", phraseSetCorrectionsTestLinter, "as soon as a infoHash query is performed, a Torrent file is retried, raising the question of:")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectArisingTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "arising the question whether the requirement of wgpu::Features::DEPTH24PLUS_STENCIL8 is precise", phraseSetCorrectionsTestLinter, "raising the question whether the requirement of wgpu::Features::DEPTH24PLUS_STENCIL8 is precise")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectRoseTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here is an example that rose the question at first: What works.", phraseSetCorrectionsTestLinter, "Here is an example that raised the question at first: What works.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectRisenTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That has risen the question in my mind if it is still possible to embed your own Flash player on Facebook today?", phraseSetCorrectionsTestLinter, "That has raised the question in my mind if it is still possible to embed your own Flash player on Facebook today?")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectRisedTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I rised the question to Emax Support and they just came back to me inmediately with the below response.", phraseSetCorrectionsTestLinter, "I raised the question to Emax Support and they just came back to me inmediately with the below response.")
}

func TestPhraseSetCorrectionsManyToManyTests_DontFagThereAroseTheQuestion(t *testing.T) {
	t.Skip("Not actually an error after when it's 'there arose'")
	linttest.AssertSuggestionResult(t, "Hello, while I have been using modals manager there arose the question related to customizing of modal header.", phraseSetCorrectionsTestLinter, "Hello, while I have been using modals manager there arose the question related to customizing of modal header.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectArisedTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and that fact arised the question in my mind, what does exactly is happening", phraseSetCorrectionsTestLinter, "and that fact raised the question in my mind, what does exactly is happening")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectAroseTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This arose the question, could I store 32 digits on the stack?", phraseSetCorrectionsTestLinter, "This raised the question, could I store 32 digits on the stack?")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectArisenTheQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Some have arisen the question like how to use this wireless HD mini camera", phraseSetCorrectionsTestLinter, "Some have raised the question like how to use this wireless HD mini camera")
}

func TestPhraseSetCorrectionsManyToManyTests_FixReversedEngineerPresent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you will save a lot of time if you don't have to reversed engineer encryption and checksums", phraseSetCorrectionsTestLinter, "you will save a lot of time if you don't have to reverse engineer encryption and checksums")
}

func TestPhraseSetCorrectionsManyToManyTests_FixReversedEngineerPast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The codes is taking the reversed engineer SEED value from GOZ family codes", phraseSetCorrectionsTestLinter, "The codes is taking the reverse engineered SEED value from GOZ family codes")
}

func TestPhraseSetCorrectionsManyToManyTests_FixReversedEngineerHyphenPast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We reversed-engineer and traced the reviews that were displayed at the time to each reviewer at the time each review was being composed.", phraseSetCorrectionsTestLinter, "We reverse-engineered and traced the reviews that were displayed at the time to each reviewer at the time each review was being composed.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixReversedEngineered(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I reversed engineered the TIDAL device authorization grant (RFC 8628) since the web flow (RFC 6749) is reCaptcha v3 secured.", phraseSetCorrectionsTestLinter, "I reverse engineered the TIDAL device authorization grant (RFC 8628) since the web flow (RFC 6749) is reCaptcha v3 secured.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixReversedEngineeredHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a reversed-engineered server for Card Wars Kingdom, designed for version 1.0.17", phraseSetCorrectionsTestLinter, "This is a reverse-engineered server for Card Wars Kingdom, designed for version 1.0.17")
}

func TestPhraseSetCorrectionsManyToManyTests_FixReversedEngineering(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Im not sure how this works but it is probably vulnerable to reversed engineering attack.", phraseSetCorrectionsTestLinter, "Im not sure how this works but it is probably vulnerable to reverse engineering attack.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixReversedEngineersVerb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... managed to create a mini-arc reactor was because had blue prints for the original and kind of reversed engineers it", phraseSetCorrectionsTestLinter, "... managed to create a mini-arc reactor was because had blue prints for the original and kind of reverse engineers it")
}

func TestPhraseSetCorrectionsManyToManyTests_FixReversedEngineersNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Expert reversed engineers with 2-15 years of industrial experience at BlueSurf Technologies.", phraseSetCorrectionsTestLinter, "Expert reverse engineers with 2-15 years of industrial experience at BlueSurf Technologies.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSeizeControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ninja token addresses were blocked from future activities to seize control over the network", phraseSetCorrectionsTestLinter, "ninja token addresses were blocked from future activities to seize control of the network")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSiezeControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The problem is that by choosing to sieze control over distribution is that they now cannot sieze control over production without breaking their ...", phraseSetCorrectionsTestLinter, "The problem is that by choosing to seize control of distribution is that they now cannot seize control of production without breaking their ...")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSeizedControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "part of a revolution in self-publishing as artists and poets seized control over the dissemination of their work", phraseSetCorrectionsTestLinter, "part of a revolution in self-publishing as artists and poets seized control of the dissemination of their work")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSiezedControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You siezed control over a large portion of the Ferelden hinterlands that you never gave back, you occupy a castle that's technically on ...", phraseSetCorrectionsTestLinter, "You seized control of a large portion of the Ferelden hinterlands that you never gave back, you occupy a castle that's technically on ...")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSeizesControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Former Brigadier-General Ansumane Mané seizes control over military barracks in Bissau, marking the beginning of the Guinea-Bissau Civil War", phraseSetCorrectionsTestLinter, "Former Brigadier-General Ansumane Mané seizes control of military barracks in Bissau, marking the beginning of the Guinea-Bissau Civil War")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSiezesControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Obsidian's mech forces the other player into coexistence and siezes control over the planet when they flip from the Firmament.", phraseSetCorrectionsTestLinter, "The Obsidian's mech forces the other player into coexistence and seizes control of the planet when they flip from the Firmament.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSeizingControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... by either shooting them or seizing control over them by creating connections in a short subgame", phraseSetCorrectionsTestLinter, "... by either shooting them or seizing control of them by creating connections in a short subgame")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSiezingControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Siezing control over the internet is the modern day equivalent of 'siezing the means of production.'.", phraseSetCorrectionsTestLinter, "Seizing control of the internet is the modern day equivalent of 'siezing the means of production.'.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSideTangentStartOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Side tangent: I personally wouldn't worry about using ; for removing the selection unless you need to.", phraseSetCorrectionsTestLinter, "Tangent: I personally wouldn't worry about using ; for removing the selection unless you need to.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSideTangentAside(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As a side tangent, in addition to not solving the gradual code repair problem", phraseSetCorrectionsTestLinter, "As an aside, in addition to not solving the gradual code repair problem")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSideTangents(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so we don't get bogged down by tiny formatting bikeshedding side tangents", phraseSetCorrectionsTestLinter, "so we don't get bogged down by tiny formatting bikeshedding tangents")
}

func TestPhraseSetCorrectionsManyToManyTests_FixABridgeTooFar(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If Winforms can ever be conquered by the Mono developers may be a bridge to far.", phraseSetCorrectionsTestLinter, "If Winforms can ever be conquered by the Mono developers may be a bridge too far.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixCakeAndEatItToo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The solution: wouldn't it be great if I could have my cake and eat it to?", phraseSetCorrectionsTestLinter, "The solution: wouldn't it be great if I could have my cake and eat it too?")
}

func TestPhraseSetCorrectionsManyToManyTests_FixGoToFar(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's difficult to be sure when we go to far sometime when you don't exactly how the beast works in the background .", phraseSetCorrectionsTestLinter, "It's difficult to be sure when we go too far sometime when you don't exactly how the beast works in the background .")
}

func TestPhraseSetCorrectionsManyToManyTests_FixGoesToFar(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Memory consumption and cpu consumption goes to far like 900% and more than this", phraseSetCorrectionsTestLinter, "Memory consumption and cpu consumption goes too far like 900% and more than this")
}

func TestPhraseSetCorrectionsManyToManyTests_FixGoingToFar(t *testing.T) {
	linttest.AssertSuggestionResult(t, "wsrun is going to far on this because debug 's devDependency shouldn't be considered in the cycle detection, should it?", phraseSetCorrectionsTestLinter, "wsrun is going too far on this because debug 's devDependency shouldn't be considered in the cycle detection, should it?")
}

func TestPhraseSetCorrectionsManyToManyTests_FixGoneToFar(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I might have gone to far with opening issues for small things.", phraseSetCorrectionsTestLinter, "I might have gone too far with opening issues for small things.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixWentToFar(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But I went to far compared to the initial request that seems talk about ...", phraseSetCorrectionsTestLinter, "But I went too far compared to the initial request that seems talk about ...")
}

func TestPhraseSetCorrectionsManyToManyTests_FixLifeSTooShort(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Life's to short for messing around with git add , writing commit message.", phraseSetCorrectionsTestLinter, "Life's too short for messing around with git add , writing commit message.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixLifesToShort(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wouldn't go back after the 3rd interview lifes to short.", phraseSetCorrectionsTestLinter, "I wouldn't go back after the 3rd interview life's too short.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixLifeIsTooShort(t *testing.T) {
	linttest.AssertSuggestionResult(t, "[Life is to short to use dated cli tools that suck]", phraseSetCorrectionsTestLinter, "[Life is too short to use dated cli tools that suck]")
}

func TestPhraseSetCorrectionsManyToManyTests_FixPutTooFineAPoint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not to put to fine a point on it... that's not the kind of team I think we want to be.", phraseSetCorrectionsTestLinter, "Not to put too fine a point on it... that's not the kind of team I think we want to be.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSpeakTooSoon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I don't want to speak to soon but I kept everything as I had before but included: http = httplib2.Http()", phraseSetCorrectionsTestLinter, "I don't want to speak too soon but I kept everything as I had before but included: http = httplib2.Http()")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSpeakingTooSoon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "EDIT: Thats what I get for speaking to soon...", phraseSetCorrectionsTestLinter, "EDIT: Thats what I get for speaking too soon...")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSpokeTooSoon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I spoke to soon. Ignore the previous post.", phraseSetCorrectionsTestLinter, "I spoke too soon. Ignore the previous post.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixSpokenTooSoon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "EDIT: I might have spoken to soon...", phraseSetCorrectionsTestLinter, "EDIT: I might have spoken too soon...")
}

func TestPhraseSetCorrectionsManyToManyTests_FixThinkTooMuch(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I don't think to much about it, but I don't think it's a big deal.", phraseSetCorrectionsTestLinter, "I don't think too much about it, but I don't think it's a big deal.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixTooBigFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ng-relations form to big for small screens", phraseSetCorrectionsTestLinter, "ng-relations form too big for small screens")
}

func TestPhraseSetCorrectionsManyToManyTests_FixTooBigToFail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The core alone has 50k LOC. Reminds me of \"to big to fail\".", phraseSetCorrectionsTestLinter, "The core alone has 50k LOC. Reminds me of \"too big to fail\".")
}

func TestPhraseSetCorrectionsManyToManyTests_FixTooGoodToBeTrue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This seemed to good to be true, but local to scene resources will not work when they are not contained in a node.", phraseSetCorrectionsTestLinter, "This seemed too good to be true, but local to scene resources will not work when they are not contained in a node.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixTooGoodTooBeTrue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The normalization of rewards is making the plot in tensorboard look too good too be true, because they are not the actual reward ...", phraseSetCorrectionsTestLinter, "The normalization of rewards is making the plot in tensorboard look too good to be true, because they are not the actual reward ...")
}

func TestPhraseSetCorrectionsManyToManyTests_FixTooMuchInformation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Live test are printing way to much information and is polluting our test output", phraseSetCorrectionsTestLinter, "Live test are printing way too much information and is polluting our test output")
}

func TestPhraseSetCorrectionsManyToManyTests_FixTooBigTooFail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In other words, pointer arithmetic is, at this point, too big too fail, regardless of the clever and sophisticated way C++ lawyercats worded it.", phraseSetCorrectionsTestLinter, "In other words, pointer arithmetic is, at this point, too big to fail, regardless of the clever and sophisticated way C++ lawyercats worded it.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectAtomicWholeEntire(t *testing.T) {
	linttest.AssertSuggestionResult(t, "whole entire", phraseSetCorrectionsTestLinter, "whole")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectRealWorldWholeEntire(t *testing.T) {
	linttest.AssertSuggestionResult(t, "[FR] support use system dns in whole entire app", phraseSetCorrectionsTestLinter, "[FR] support use system dns in whole app")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectAtomicAWholeEntireToAWhole(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a whole entire", phraseSetCorrectionsTestLinter, "a whole")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectAtomicAWholeEntireToAnEntire(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a whole entire", phraseSetCorrectionsTestLinter, "an entire")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectRealWorldAWholeEntireToAWhole(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Start mapping a whole entire new planet using NASA’s MOLA.", phraseSetCorrectionsTestLinter, "Start mapping a whole new planet using NASA’s MOLA.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectRealWorldAWholeEntireToAnEntire(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am not sure I can pass in a whole entire query via the include.", phraseSetCorrectionsTestLinter, "I am not sure I can pass in an entire query via the include.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectALotWorseAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a lot worst", phraseSetCorrectionsTestLinter, "a lot worse")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectALotWorseRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "On a debug build, it's even a lot worst.", phraseSetCorrectionsTestLinter, "On a debug build, it's even a lot worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBecameWorst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The problem became worst lately.", phraseSetCorrectionsTestLinter, "The problem became worse lately.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBecomeWorst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But results seems stay at one place or become worst.", phraseSetCorrectionsTestLinter, "But results seems stay at one place or become worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBecomesWorst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This becomes worst if you have an x64 dll and an x86 dll that you don't have thier source codes and want to use them in same project!", phraseSetCorrectionsTestLinter, "This becomes worse if you have an x64 dll and an x86 dll that you don't have thier source codes and want to use them in same project!")
}

func TestPhraseSetCorrectionsManyToManyTests_FixBecomingWorst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "France is becoming worst than the Five Eyes", phraseSetCorrectionsTestLinter, "France is becoming worse than the Five Eyes")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectFarWorseAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "far worst", phraseSetCorrectionsTestLinter, "far worse")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectFarWorseRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I mainly use Firefox (personal preference) and have noticed it has far worst performance than Chrome", phraseSetCorrectionsTestLinter, "I mainly use Firefox (personal preference) and have noticed it has far worse performance than Chrome")
}

func TestPhraseSetCorrectionsManyToManyTests_FixGetWorse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and the problem appears to get worst with 2025.5.1 and 2025.5.2.", phraseSetCorrectionsTestLinter, "and the problem appears to get worse with 2025.5.1 and 2025.5.2.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixGetsWorse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It just starts after about 15 minutes of work and gradually gets worst.", phraseSetCorrectionsTestLinter, "It just starts after about 15 minutes of work and gradually gets worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_DontFlagGettingWorst(t *testing.T) {
	t.Skip("This kind of false positive is probably too subtle to detect")
	linttest.AssertLintCount(t, "I am getting worst accuracy on the same dataste and 3 different models.", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsManyToManyTests_FixGettingWorst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But, as I said, it is getting worst...", phraseSetCorrectionsTestLinter, "But, as I said, it is getting worse...")
}

func TestPhraseSetCorrectionsManyToManyTests_FixGotWorst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "typescript support got worst.", phraseSetCorrectionsTestLinter, "typescript support got worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_FixGottenWorst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Has Claude gotten worst?", phraseSetCorrectionsTestLinter, "Has Claude gotten worse?")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMuchWorseAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "much worst", phraseSetCorrectionsTestLinter, "much worse")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMuchWorseRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the generated image quality is much worst (actually nearly broken)", phraseSetCorrectionsTestLinter, "the generated image quality is much worse (actually nearly broken)")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectTurnForTheWorseAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "turn for the worst", phraseSetCorrectionsTestLinter, "turn for the worse")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectTurnForTheWorseRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Very surprised to see this repo take such a turn for the worst.", phraseSetCorrectionsTestLinter, "Very surprised to see this repo take such a turn for the worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectWorseThanAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "worst than", phraseSetCorrectionsTestLinter, "worse than")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectWorseThanRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Project real image - inversion quality is worst than in StyleGAN2", phraseSetCorrectionsTestLinter, "Project real image - inversion quality is worse than in StyleGAN2")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectWorstEverAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "worse ever", phraseSetCorrectionsTestLinter, "worst ever")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectWorstEverRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Bcl package family is one of the worse ever published by Microsoft.", phraseSetCorrectionsTestLinter, "The Bcl package family is one of the worst ever published by Microsoft.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectWorstAndWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "worst and worst", phraseSetCorrectionsTestLinter, "worse and worse")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectWorstAndWorstRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This control-L trick does not work for me. The padding is getting worst and worst.", phraseSetCorrectionsTestLinter, "This control-L trick does not work for me. The padding is getting worse and worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectWorseAndWorstRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This progressively got worse and worst to the point that the machine (LEAD 1010) stopped moving alltogether.", phraseSetCorrectionsTestLinter, "This progressively got worse and worse to the point that the machine (LEAD 1010) stopped moving alltogether.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectAtWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Partial moving of core objects to interpreter state is incorrect at best, unsafe at worse.", phraseSetCorrectionsTestLinter, "Partial moving of core objects to interpreter state is incorrect at best, unsafe at worst.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectTheWorseThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A 16-32 decoder is pretty simple, a 16-32-48 isn't the worse thing in the world", phraseSetCorrectionsTestLinter, "A 16-32 decoder is pretty simple, a 16-32-48 isn't the worst thing in the world")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectWorseCaseSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the worse case scenario, remote code execution could be achieved.", phraseSetCorrectionsTestLinter, "In the worst-case scenario, remote code execution could be achieved.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectWorseCaseHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Basically I want my pods to get the original client IP address... or at least have X-Forwarded-For header, in a worse-case scenario.", phraseSetCorrectionsTestLinter, "Basically I want my pods to get the original client IP address... or at least have X-Forwarded-For header, in a worst-case scenario.")
}

func TestPhraseSetCorrectionsManyToManyTests_CorrectWorseCaseTwoHyphens(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In a worse-case-scenario, the scenario class code and the results being analysed, become out of sync, and so the wrong labels are applied.", phraseSetCorrectionsTestLinter, "In a worst-case scenario, the scenario class code and the results being analysed, become out of sync, and so the wrong labels are applied.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMakeItWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And if you try to access before that, CloudFront will cache the error and it'll make it worst.", phraseSetCorrectionsTestLinter, "And if you try to access before that, CloudFront will cache the error and it'll make it worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMadeItWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However in couple of occasions the refresh made it worst and it showed commit differences that were already commited and pushed to origin.", phraseSetCorrectionsTestLinter, "However in couple of occasions the refresh made it worse and it showed commit differences that were already commited and pushed to origin.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMakesItWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What makes it worst, is if I use the returned SHA to try and update the newly created file I get the same error I show below.", phraseSetCorrectionsTestLinter, "What makes it worse, is if I use the returned SHA to try and update the newly created file I get the same error I show below.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMakingItWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "PLease ai realled need help with this I think I'm making it worst.", phraseSetCorrectionsTestLinter, "PLease ai realled need help with this I think I'm making it worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMakeThemWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not sure if this makes things clearer or make them worst.", phraseSetCorrectionsTestLinter, "Not sure if this makes things clearer or make them worse.")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMadeThemWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if not outroght caused them / made them worst", phraseSetCorrectionsTestLinter, "if not outroght caused them / made them worse")
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMakesThemWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "(tried ~14 different hyperparameter and data format combos), however, always just makes them worst, they go from \"slightly\" wrong to \"complete nonsense\".", phraseSetCorrectionsTestLinter, "(tried ~14 different hyperparameter and data format combos), however, always just makes them worse, they go from \"slightly\" wrong to \"complete nonsense\".")
}

func TestPhraseSetCorrectionsManyToManyTests_DontFlagMakesThemWorstCase(t *testing.T) {
	t.Skip("This false positive is not handled yet")
	linttest.AssertLintCount(t, "Note 1: all hash tables has an Achilles heel that makes them worst case O(N)", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsManyToManyTests_DetectMakingThemWorstAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As for the last part about Apple deliberately making them worst in order for us to buy the 3s", phraseSetCorrectionsTestLinter, "As for the last part about Apple deliberately making them worse in order for us to buy the 3s")
}

var phraseSetCorrectionsTestLinter = lint.NewPooledLinter(NewPhraseSetCorrectionsLintGroup)

func phraseSetCorrectionsSingleLint(ruleName string) *lint.LintGroup {
	group := NewPhraseSetCorrectionsLintGroup()
	group.SetAllRulesTo(nil)
	group.Config.SetRuleEnabled(ruleName, true)
	return group
}
