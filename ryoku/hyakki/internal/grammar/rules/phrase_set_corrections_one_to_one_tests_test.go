package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPhraseSetCorrectionsOneToOneTests_CorrectsFurtherAdo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... but we finally hit a great spot, so without further adieu.", phraseSetCorrectionsTestLinter, "... but we finally hit a great spot, so without further ado.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsMuchAdo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "After much adieu this functionality is now available.", phraseSetCorrectionsTestLinter, "After much ado this functionality is now available.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsTheresAnArgumentToBeSaid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I guess there s an argument to be said that if the TUCKR_HOME is already defined, it should use that instead.", phraseSetCorrectionsTestLinter, "I guess there s an argument to be made that if the TUCKR_HOME is already defined, it should use that instead.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsThereIsAnArgumentToBeSaid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Same argument for smooth_image_crate, although there is an argument to be said this is more-generally useful than scale_image_crate", phraseSetCorrectionsTestLinter, "Same argument for smooth_image_crate, although there is an argument to be made this is more-generally useful than scale_image_crate")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsTheresTheresArgumentsToBeSaid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "there's there's arguments to be said for all of it.", phraseSetCorrectionsTestLinter, "there's there's arguments to be made for all of it.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixCompleteBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "why you think some of them are complete bullocks or would be a bad idea", phraseSetCorrectionsTestLinter, "why you think some of them are complete bollocks or would be a bad idea")
}

func TestPhraseSetCorrectionsOneToOneTests_FixDogs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The cat's ass, priceless! I have to steal that one. My go to phrase is “The dog's bullocks.", phraseSetCorrectionsTestLinter, "The cat's ass, priceless! I have to steal that one. My go to phrase is “The dog's bollocks.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixDogsNoApostropheBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "some dumb rubbish that i do not give a dogs bullocks about", phraseSetCorrectionsTestLinter, "some dumb rubbish that i do not give a dogs bollocks about")
}

func TestPhraseSetCorrectionsOneToOneTests_FixIsBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "for me this is bullocks, when the same user can sudo rm -rf", phraseSetCorrectionsTestLinter, "for me this is bollocks, when the same user can sudo rm -rf")
}

func TestPhraseSetCorrectionsOneToOneTests_FixItsBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm too lazy to explain why, but I think it's bullocks.", phraseSetCorrectionsTestLinter, "I'm too lazy to explain why, but I think it's bollocks.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixItsNoApostropheBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but lance, dont claim to be clean, because we all know its bullocks", phraseSetCorrectionsTestLinter, "but lance, dont claim to be clean, because we all know its bollocks")
}

func TestPhraseSetCorrectionsOneToOneTests_FixSuchBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is why numerology is such bullocks.", phraseSetCorrectionsTestLinter, "This is why numerology is such bollocks.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixThatsBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Respectfully, that's bullocks.", phraseSetCorrectionsTestLinter, "Respectfully, that's bollocks.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixThatsNoApostropheBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In CSS thats bullocks as directives have priority in the order they are defined.", phraseSetCorrectionsTestLinter, "In CSS thats bollocks as directives have priority in the order they are defined.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTotalBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Pointing out to the audience that their gravity explanation is total bullocks would seem an ethical must as well.", phraseSetCorrectionsTestLinter, "Pointing out to the audience that their gravity explanation is total bollocks would seem an ethical must as well.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixUtterBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "what utter bullocks a self employed person will get £94 under corona virus crisis", phraseSetCorrectionsTestLinter, "what utter bollocks a self employed person will get £94 under corona virus crisis")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWasBullocks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a few years ago I thought that was bullocks", phraseSetCorrectionsTestLinter, "a few years ago I thought that was bollocks")
}

func TestPhraseSetCorrectionsOneToOneTests_FixBullocksExclamation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "throw(new Error('Bullocks!')));", phraseSetCorrectionsTestLinter, "throw(new Error('Bollocks!')));")
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagHerdOfBullocks(t *testing.T) {
	linttest.AssertNoLints(t, "driven back (literally) by a herd of bullocks across the path", phraseSetCorrectionsTestLinter)
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectChompAtTheBit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so other than rolling back to older drivers i might have to chomp at the bit for a while longer yet", phraseSetCorrectionsTestLinter, "so other than rolling back to older drivers i might have to champ at the bit for a while longer yet")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectChompedAtTheBit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I chomped at the bit, frustrated by my urge to go faster, while my husband chafed at what I thought was a moderate pace.", phraseSetCorrectionsTestLinter, "I champed at the bit, frustrated by my urge to go faster, while my husband chafed at what I thought was a moderate pace.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectChompingAtTheBit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Checking in to see when the Windows install will be ready. I am chomping at the bit!", phraseSetCorrectionsTestLinter, "Checking in to see when the Windows install will be ready. I am champing at the bit!")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectChompsAtTheBit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nobody chomps at the bit to make sure these are maintained, current, complete, and error free", phraseSetCorrectionsTestLinter, "nobody champs at the bit to make sure these are maintained, current, complete, and error free")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectClientsSide(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I want to debug this server-side as I cannot find out why the connection is being refused from the client's side.", phraseSetCorrectionsTestLinter, "I want to debug this server-side as I cannot find out why the connection is being refused from the client-side.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectServersSide(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A client-server model where the client can execute commands in a terminal on the server's side", phraseSetCorrectionsTestLinter, "A client-server model where the client can execute commands in a terminal on the server-side")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCombinate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm at chapter 11 and I can't craft and combinate abyss gear, what should I do to unlock both of them", phraseSetCorrectionsTestLinter, "I'm at chapter 11 and I can't craft and combine abyss gear, what should I do to unlock both of them")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCombinated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "NFS WORLD COMBINATED MAP (NEW!)", phraseSetCorrectionsTestLinter, "NFS WORLD COMBINED MAP (NEW!)")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCombinates(t *testing.T) {
	linttest.AssertSuggestionResult(t, "is there a game that combinates ottd and rts?", phraseSetCorrectionsTestLinter, "is there a game that combines ottd and rts?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCombinating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This section discusses how the color combinating is accomplished", phraseSetCorrectionsTestLinter, "This section discusses how the color combining is accomplished")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCondensated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Having this info condensated in the package list would help to draw conclusions whether some packages need upgrade.", phraseSetCorrectionsTestLinter, "Having this info condensed in the package list would help to draw conclusions whether some packages need upgrade.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCondensating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Apparently this is needed to minimize the risk of having water condensating.", phraseSetCorrectionsTestLinter, "Apparently this is needed to minimize the risk of having water condensing.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCompulse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Play Store will soon compulse to use SDK 30 on any app updates , and it's mandatory to have SDK 30 for new apps.", phraseSetCorrectionsTestLinter, "Play Store will soon compel to use SDK 30 on any app updates , and it's mandatory to have SDK 30 for new apps.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCompulsed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just alpha, but now i am compulsed to work 10.6 into the github actions and insane docker environment :)", phraseSetCorrectionsTestLinter, "Just alpha, but now i am compelled to work 10.6 into the github actions and insane docker environment :)")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCompulses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Occasionally, a film comes along that compulses me to make a fan poster.", phraseSetCorrectionsTestLinter, "Occasionally, a film comes along that compels me to make a fan poster.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCompulsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have an button enabled to prompt user to download the app whenever we find difference in version number in our servlet war file and apk verision compulsing user to update.", phraseSetCorrectionsTestLinter, "We have an button enabled to prompt user to download the app whenever we find difference in version number in our servlet war file and apk verision compelling user to update.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectConformThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the WCAG requires every view of the page to conform that we move this", phraseSetCorrectionsTestLinter, "the WCAG requires every view of the page to confirm that we move this")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsConformedThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have conformed that works now.", phraseSetCorrectionsTestLinter, "I have confirmed that works now.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsConformsThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I conformed that with the correct configuration, this is working correctly.", phraseSetCorrectionsTestLinter, "I confirmed that with the correct configuration, this is working correctly.")
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagConformingThat(t *testing.T) {
	t.Skip("False positive not yet handled.")
	linttest.AssertLintCount(t, "is there any example of a case that isn't fully conforming that is supported today?", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsConformingThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thanks for conforming that this issue is fixed in the latest version.", phraseSetCorrectionsTestLinter, "Thanks for confirming that this issue is fixed in the latest version.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsConstituteAs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This doesn't really constitute as an implicit cast in the eyes of the system.", phraseSetCorrectionsTestLinter, "This doesn't really constitute an implicit cast in the eyes of the system.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsConstitutedAs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We do not recommend setting the number of threads to more than 20, as that can be constituted as a denial of service attack which we are not responsible for.", phraseSetCorrectionsTestLinter, "We do not recommend setting the number of threads to more than 20, as that can be constituted a denial of service attack which we are not responsible for.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsConstitutesAs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hello! I was just wondering what constitutes as a prompt in GitHub CoPilot that consumes premium request tokens.", phraseSetCorrectionsTestLinter, "Hello! I was just wondering what constitutes a prompt in GitHub CoPilot that consumes premium request tokens.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsConstitutingAsExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is the example constituting as hull-demo 's values.yaml", phraseSetCorrectionsTestLinter, "This is the example constituting hull-demo 's values.yaml")
}

func TestPhraseSetCorrectionsOneToOneTests_AmbiguousConstitutingAs(t *testing.T) {
	t.Skip("Not sure if this would be a false positive or a true positive")
	linttest.AssertSuggestionResult(t, "Note that spinning up a Client is a non-trivial operation, constituting as much as a millisecond of overhead.", phraseSetCorrectionsTestLinter, "Note that spinning up a Client is a non-trivial operation, constituting as much as a millisecond of overhead.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDefiniteArticle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As for format of outputs: the spec defines the field as using the singular definitive article \"the\"", phraseSetCorrectionsTestLinter, "As for format of outputs: the spec defines the field as using the singular definite article \"the\"")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDefiniteArticlesTitleCase(t *testing.T) {
	t.Skip("Title case capitalization problem causes this one to fail too.")
	linttest.AssertSuggestionResult(t, "01 Definitive Articles: De or Het. Before starting more complicated topics in Dutch grammar, you should be aware of the articles.", phraseSetCorrectionsTestLinter, "01 Definite Articles: De or Het. Before starting more complicated topics in Dutch grammar, you should be aware of the articles.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDefiniteArticlesLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, ".. definitive articles -та /-ta/ and -те /-te/ (postfixed in Bulgarian).", phraseSetCorrectionsTestLinter, ".. definite articles -та /-ta/ and -те /-te/ (postfixed in Bulgarian).")
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagDigestiveTrack(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In infants less than a year old, because their digestive track is not finished developing yet", phraseSetCorrectionsTestLinter, "In infants less than a year old, because their digestive tract is not finished developing yet")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDigestiveTracks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The digestive tracks of mammals are complex and diverse, with each species having its own unique digestive system.", phraseSetCorrectionsTestLinter, "The digestive tracts of mammals are complex and diverse, with each species having its own unique digestive system.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDoseNot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It dose not run windows ?", phraseSetCorrectionsTestLinter, "It does not run windows ?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDoseIt(t *testing.T) {
	t.Skip("due to false positives this can't be fixed yet")
	linttest.AssertSuggestionResult(t, "dose it support zh_cn ？", phraseSetCorrectionsTestLinter, "does it support zh_cn ？")
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagExcessiveDoseItMight(t *testing.T) {
	t.Skip("would be a false positive in a naive implementation")
	linttest.AssertLintCount(t, "it should be noted that in an excessive dose it might have an opposite effect", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagSecondDoseItIsNot(t *testing.T) {
	t.Skip("would be a false positive in a naive implementation")
	linttest.AssertLintCount(t, "When the person receives a prescribed second dose it is not counted ttwice", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagADoseItWas(t *testing.T) {
	t.Skip("would be a false positive in a naive implementation")
	linttest.AssertLintCount(t, "At that small a dose it was pleasent.", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagWhatDoseItTakes(t *testing.T) {
	t.Skip("would be a false positive in a naive implementation")
	linttest.AssertLintCount(t, "I do not know what dose it takes to trip out, but I don't think I could stay awake to find out.", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagToDoseIt(t *testing.T) {
	t.Skip("would be a false positive in a naive implementation")
	linttest.AssertLintCount(t, "And then I have to re-add the salts back to it to dose it back up to drinkable.", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagDontDoseItTooHigh(t *testing.T) {
	t.Skip("would be a false positive in a naive implementation")
	linttest.AssertLintCount(t, "So my conclusion is: don't dose it too high or it actually is dangerous and not pleasant at all", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_DontFlagToDoseItOff(t *testing.T) {
	t.Skip("would be a false positive in a naive implementation")
	linttest.AssertLintCount(t, "the only solution the other hopefully-dominant-reasonable-adult-human mind can find, is to dose it off, hoping the drowsiness can keep the fear at bay", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsHeDoes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This validate each and every field of your from with nice dotted red color warring for the user, incase he dose some mistakes.", phraseSetCorrectionsTestLinter, "This validate each and every field of your from with nice dotted red color warring for the user, incase he does some mistakes.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsSheDoes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we wont agree on everything she dose thats what a real person would feel like", phraseSetCorrectionsTestLinter, "we wont agree on everything she does thats what a real person would feel like")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsItDose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it dose work without WEBP enabled", phraseSetCorrectionsTestLinter, "it does work without WEBP enabled")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsSomeoneDose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hopefully someone dose, I'm not good at C programing....", phraseSetCorrectionsTestLinter, "Hopefully someone does, I'm not good at C programing....")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsHowDose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How dose qsv-copy works?", phraseSetCorrectionsTestLinter, "How does qsv-copy works?")
}

func TestPhraseSetCorrectionsOneToOneTests_DontFixHowDoseFalsePositive(t *testing.T) {
	t.Skip("false positive not yet detected")
	linttest.AssertLintCount(t, "Work in progress exploration of how dose modifications throughout a trial can also induce bias in the exposure-response relationships.", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhatDoseA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What dose a business analyst do?", phraseSetCorrectionsTestLinter, "What does a business analyst do?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhatDoseAn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What dose an ear infection feel like?", phraseSetCorrectionsTestLinter, "What does an ear infection feel like?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhatDoseIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What dose it mean when a cat wags its tail?", phraseSetCorrectionsTestLinter, "What does it mean when a cat wags its tail?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhatDoseThis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What dose this sign mean?", phraseSetCorrectionsTestLinter, "What does this sign mean?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhatDoseThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What dose that mean?", phraseSetCorrectionsTestLinter, "What does that mean?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhatDoseThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What dose the term gaslighting mean", phraseSetCorrectionsTestLinter, "What does the term gaslighting mean")
}

func TestPhraseSetCorrectionsOneToOneTests_AllowsWhatDoseOf(t *testing.T) {
	linttest.AssertLintCount(t, "What dose of vitamin d should I take?", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_AllowsWhatDoseIs(t *testing.T) {
	linttest.AssertLintCount(t, "What dose is considered safe?", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_DontFixWhatDoseItFalsePositive(t *testing.T) {
	t.Skip("false positive not yet detected")
	linttest.AssertLintCount(t, "I do not know what dose it takes to trip out, but I don't think I could stay awake to find out.", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhenDose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When dose reusebale variable sync between device? #2634", phraseSetCorrectionsTestLinter, "When does reusebale variable sync between device? #2634")
}

func TestPhraseSetCorrectionsOneToOneTests_DontFixWhenDoseFalsePositive(t *testing.T) {
	t.Skip("false positive not yet detected")
	linttest.AssertLintCount(t, "Should we remove the dose when dose has been applied", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhereDose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "where dose the password store?", phraseSetCorrectionsTestLinter, "where does the password store?")
}

func TestPhraseSetCorrectionsOneToOneTests_DontFixWhereDoseFalsePositive(t *testing.T) {
	t.Skip("false positive not yet detected")
	linttest.AssertLintCount(t, "added some better error handling for the weird case where dose files have no dose...", phraseSetCorrectionsTestLinter, 0)
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhoDose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Who dose knows the problem?", phraseSetCorrectionsTestLinter, "Who does knows the problem?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsWhyDose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "why dose the path is random ?", phraseSetCorrectionsTestLinter, "why does the path is random ?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDabbleDown(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The MCU need to dabble down on jerk Xavier.", phraseSetCorrectionsTestLinter, "The MCU need to double down on jerk Xavier.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDabbledDown(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it really was the perfect pose and everybody dabbled down on that when they did switch from heel", phraseSetCorrectionsTestLinter, "it really was the perfect pose and everybody doubled down on that when they did switch from heel")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDabblesDown(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I already loved but this game really dabbles down on the obscurity", phraseSetCorrectionsTestLinter, "I already loved but this game really doubles down on the obscurity")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDabblingDown(t *testing.T) {
	linttest.AssertSuggestionResult(t, "learn how to respond with clarity, not dabbling down on your price", phraseSetCorrectionsTestLinter, "learn how to respond with clarity, not doubling down on your price")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsAlgo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Always glad when the algo feeds me a new dissident.", phraseSetCorrectionsTestLinter, "Always glad when the algorithm feeds me a new dissident.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsAlgos(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I moved algos development to a private repository.", phraseSetCorrectionsTestLinter, "I moved algorithms development to a private repository.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsArg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but I cannot figure out how to flag an arg as required", phraseSetCorrectionsTestLinter, "but I cannot figure out how to flag an argument as required")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsArgs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but every test I've done shows args as being about 65% faster", phraseSetCorrectionsTestLinter, "but every test I've done shows arguments as being about 65% faster")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsCoord(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Prompted by #5684, we should probably emit more meaningful messages when position guides are specified in coord systems that do not support them", phraseSetCorrectionsTestLinter, "Prompted by #5684, we should probably emit more meaningful messages when position guides are specified in coordinate systems that do not support them")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsCoords(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here is how you can extract the list of coords from any geometry:", phraseSetCorrectionsTestLinter, "Here is how you can extract the list of coordinates from any geometry:")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDecl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yeah, I agree a forward decl would be preferable in this case.", phraseSetCorrectionsTestLinter, "Yeah, I agree a forward declaration would be preferable in this case.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDecls(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Accessing type decls from pointer types", phraseSetCorrectionsTestLinter, "Accessing type declarations from pointer types")
}

func TestPhraseSetCorrectionsOneToOneTests_ExpandDeref(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Should raw pointer deref/projections have to be in-bounds?", phraseSetCorrectionsTestLinter, "Should raw pointer dereference/projections have to be in-bounds?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsDerefs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A contiguous-in-memory double-ended queue that derefs into a slice - gnzlbg/slice_deque.", phraseSetCorrectionsTestLinter, "A contiguous-in-memory double-ended queue that dereferences into a slice - gnzlbg/slice_deque.")
}

func TestPhraseSetCorrectionsOneToOneTests_ExpandsDir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Error: library dir does not exist: /Users/u/trr/node_modules/opencv", phraseSetCorrectionsTestLinter, "Error: library directory does not exist: /Users/u/trr/node_modules/opencv")
}

func TestPhraseSetCorrectionsOneToOneTests_ExpandsDirs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Dirs/files are missing when scanning on windows after 1.27.12", phraseSetCorrectionsTestLinter, "Directories/files are missing when scanning on windows after 1.27.12")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsNotif(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Amazing to see the notif of this on my phone!", phraseSetCorrectionsTestLinter, "Amazing to see the notification of this on my phone!")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsNotifs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I don't encourage you spending all your time on social media or keeping the notifs on if you're working on something serious.", phraseSetCorrectionsTestLinter, "I don't encourage you spending all your time on social media or keeping the notifications on if you're working on something serious.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsParam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If I use the following to set an endDate param with a default value", phraseSetCorrectionsTestLinter, "If I use the following to set an endDate parameter with a default value")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsParams(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the params are not loaded in the R environment when using the terminal", phraseSetCorrectionsTestLinter, "the parameters are not loaded in the R environment when using the terminal")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsVuln(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I did not understand this vuln in first place now I do not understand in 2nd place as well😢", phraseSetCorrectionsTestLinter, "I did not understand this vulnerability in first place now I do not understand in 2nd place as well😢")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsVulns(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... when persisted, containing endpoints, vulns, WAF bypasses, sensitive params, and auth endpoints.", phraseSetCorrectionsSingleLint("ExpandVulnerability"), "... when persisted, containing endpoints, vulnerabilities, WAF bypasses, sensitive params, and auth endpoints.")
	linttest.AssertSuggestionResult(t, "... when persisted, containing endpoints, vulns, WAF bypasses, sensitive params, and auth endpoints.", phraseSetCorrectionsTestLinter, "... when persisted, containing endpoints, vulnerabilities, WAF bypasses, sensitive parameters, and auth endpoints.")
}

func TestPhraseSetCorrectionsOneToOneTests_DetectExplanationMarkAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "explanation mark", phraseSetCorrectionsTestLinter, "exclamation mark")
}

func TestPhraseSetCorrectionsOneToOneTests_DetectExplanationMarksAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "explanation marks", phraseSetCorrectionsTestLinter, "exclamation marks")
}

func TestPhraseSetCorrectionsOneToOneTests_DetectExplanationMarkRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Note that circled explanation mark, question mark, plus and arrows may be significantly harder to distinguish than their uncircled variants.", phraseSetCorrectionsTestLinter, "Note that circled exclamation mark, question mark, plus and arrows may be significantly harder to distinguish than their uncircled variants.")
}

func TestPhraseSetCorrectionsOneToOneTests_DetectExplanationMarksRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this issue: html: properly handle explanation marks in comments", phraseSetCorrectionsTestLinter, "this issue: html: properly handle exclamation marks in comments")
}

func TestPhraseSetCorrectionsOneToOneTests_DetectExplanationPointAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "explanation point", phraseSetCorrectionsTestLinter, "exclamation point")
}

func TestPhraseSetCorrectionsOneToOneTests_DetectExplanationPointRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "js and makes an offhand mention that you can disable inbuilt plugin with an explanation point (e.g. !error ).", phraseSetCorrectionsTestLinter, "js and makes an offhand mention that you can disable inbuilt plugin with an exclamation point (e.g. !error ).")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectCertainExtend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a PowerShell script to automate client pentests / checkups - at least to a certain extend.", phraseSetCorrectionsTestLinter, "This is a PowerShell script to automate client pentests / checkups - at least to a certain extent.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectToTheExtend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our artifacts are carefully documented and well-structured to the extend that reuse is facilitated.", phraseSetCorrectionsTestLinter, "Our artifacts are carefully documented and well-structured to the extent that reuse is facilitated.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectToSomeExtend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi, I'm new to Pydantic and to some extend python, and I have a question that I haven't been able to figure out from the Docs.", phraseSetCorrectionsTestLinter, "Hi, I'm new to Pydantic and to some extent python, and I have a question that I haven't been able to figure out from the Docs.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectToAnExtend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It mimics (to an extend) the way in which Chrome requests SSO cookies with the Windows 10 accounts extension.", phraseSetCorrectionsTestLinter, "It mimics (to an extent) the way in which Chrome requests SSO cookies with the Windows 10 accounts extension.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectLesserExtend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Unlike the US (and to a lesser extend, Europe), Chinese do not have a majority of their wealth invested in stocks.", phraseSetCorrectionsTestLinter, "Unlike the US (and to a lesser extent, Europe), Chinese do not have a majority of their wealth invested in stocks.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectGreaterExtend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "which seems to lead to columns being loaded to a greater extend than necessary", phraseSetCorrectionsTestLinter, "which seems to lead to columns being loaded to a greater extent than necessary")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsFlauntTheRules(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Some users flaunt the rules of punctuation.", phraseSetCorrectionsTestLinter, "Some users flout the rules of punctuation.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsFlauntedTheLaw(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He flaunted the law for personal gain.", phraseSetCorrectionsTestLinter, "He flouted the law for personal gain.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsFlauntingAuthority(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She was flaunting authority at every turn.", phraseSetCorrectionsTestLinter, "She was flouting authority at every turn.")
}

func TestPhraseSetCorrectionsOneToOneTests_AllowsFlauntWealth(t *testing.T) {
	linttest.AssertNoLints(t, "He likes to flaunt his wealth.", phraseSetCorrectionsTestLinter)
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectFoamOutTheMouth(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and he gave him a drink that made him foam out the mouth and die", phraseSetCorrectionsTestLinter, "and he gave him a drink that made him foam at the mouth and die")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectFoamedOutTheMouth(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can see in some shots they've foamed out the mouth, and it's apparent their poisoned.", phraseSetCorrectionsTestLinter, "You can see in some shots they've foamed at the mouth, and it's apparent their poisoned.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectFoamingOutTheMouth(t *testing.T) {
	linttest.AssertSuggestionResult(t, "choking or foaming out the mouth or something like that, leading up to death", phraseSetCorrectionsTestLinter, "choking or foaming at the mouth or something like that, leading up to death")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectFoamsOutTheMouth(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Elaine can't swallow, foams out the mouth and Kramer says she has rabies just like his friend Bob Sacamano after she gets bit by the guy's dog", phraseSetCorrectionsTestLinter, "Elaine can't swallow, foams at the mouth and Kramer says she has rabies just like his friend Bob Sacamano after she gets bit by the guy's dog")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectFlipTheBill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "- SQL Compare (If the company will flip the bill)", phraseSetCorrectionsTestLinter, "- SQL Compare (If the company will foot the bill)")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectFlippedTheBill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As a meetup we were extremely lucky that NOVI flipped the bill for our in-person events.", phraseSetCorrectionsTestLinter, "As a meetup we were extremely lucky that NOVI footed the bill for our in-person events.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectFlippingTheBill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "for the simple reason that there were no multimillion dollar company flipping the bill", phraseSetCorrectionsTestLinter, "for the simple reason that there were no multimillion dollar company footing the bill")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectFlipsTheBill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There seems to be a perennial debate in Illinois between urbanites and rural folk about who really flips the bill.", phraseSetCorrectionsTestLinter, "There seems to be a perennial debate in Illinois between urbanites and rural folk about who really foots the bill.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGetUsedOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am following the examples in the documentation in order to get used of comets.", phraseSetCorrectionsTestLinter, "I am following the examples in the documentation in order to get used to comets.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGetsUsedOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "its like she gets used of her food and becomes spoiled", phraseSetCorrectionsTestLinter, "its like she gets used to her food and becomes spoiled")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGettingUsedOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here you can find a guide to getting used of the most important methods of magum.", phraseSetCorrectionsTestLinter, "Here you can find a guide to getting used to the most important methods of magum.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGotUsedOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we users actually got used of such delays", phraseSetCorrectionsTestLinter, "we users actually got used to such delays")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGottenUsedOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The tutorial has indeed been of help, and I've gotten used of using Hull.", phraseSetCorrectionsTestLinter, "The tutorial has indeed been of help, and I've gotten used to using Hull.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGrindToHalt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Without this it will eventually grind to halt as it backs up upon itself", phraseSetCorrectionsTestLinter, "Without this it will eventually grind to a halt as it backs up upon itself")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGrindToHaltTitleCase(t *testing.T) {
	t.Skip("Fails due to how replace_with_matched_case works")
	linttest.AssertSuggestionResult(t, "Smart Search Tools Cause System to Grind to Halt", phraseSetCorrectionsTestLinter, "Smart Search Tools Cause System to Grind to a Halt")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGrindingToHalt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "app grinding to halt when loading many objects", phraseSetCorrectionsTestLinter, "app grinding to a halt when loading many objects")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGrindsToHalt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If your machine grinds to halt due to memory oversubscription, you may want to try to set the MOLD_JOBS environment variable to 1", phraseSetCorrectionsTestLinter, "If your machine grinds to a halt due to memory oversubscription, you may want to try to set the MOLD_JOBS environment variable to 1")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGroundToHalt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As you have probably guessed, my work on my fork has ground to halt.", phraseSetCorrectionsTestLinter, "As you have probably guessed, my work on my fork has ground to a halt.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHasPast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Track the amount of time that has past since a point in time.", phraseSetCorrectionsTestLinter, "Track the amount of time that has passed since a point in time.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHavePast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Another 14+ days have past, any updates on this?", phraseSetCorrectionsTestLinter, "Another 14+ days have passed, any updates on this?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHadPast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Few days had past, so im starting to thinks there is a problem in my local version.", phraseSetCorrectionsTestLinter, "Few days had passed, so im starting to thinks there is a problem in my local version.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHavingPast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Return to computer, with enough time having past for the computer to go to full sleep.", phraseSetCorrectionsTestLinter, "Return to computer, with enough time having passed for the computer to go to full sleep.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHitTheNail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ahh, found it! You hit the nail in the head once again.", phraseSetCorrectionsTestLinter, "Ahh, found it! You hit the nail on the head once again.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHitsTheNail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm not sure if this sentence hits the nail in the head", phraseSetCorrectionsTestLinter, "I'm not sure if this sentence hits the nail on the head")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHittingTheNail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You are hitting the nail in the head of my issue with this game, too.", phraseSetCorrectionsTestLinter, "You are hitting the nail on the head of my issue with this game, too.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHittedTheNail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I mean, you just kinda hitted the nail in the head. You cannot do anything with this that you couldn't do in a Raspberry PI.", phraseSetCorrectionsTestLinter, "I mean, you just kinda hitted the nail on the head. You cannot do anything with this that you couldn't do in a Raspberry PI.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHoneInOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This way you can use an object detector algorithm to hone in on subjects and tell sam to only focus in certain areas when looking to extend ...", phraseSetCorrectionsTestLinter, "This way you can use an object detector algorithm to home in on subjects and tell sam to only focus in certain areas when looking to extend ...")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHoningInOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think I understand the syntax limitation you're honing in on.", phraseSetCorrectionsTestLinter, "I think I understand the syntax limitation you're homing in on.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHonesInOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "[FEATURE] Add a magnet that hones in on mobs", phraseSetCorrectionsTestLinter, "[FEATURE] Add a magnet that homes in on mobs")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectHonedInOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But it took me quite a bit of faffing about checking things out before I honed in on the session as the problem and tried to dump out the ...", phraseSetCorrectionsTestLinter, "But it took me quite a bit of faffing about checking things out before I homed in on the session as the problem and tried to dump out the ...")
}

func TestPhraseSetCorrectionsOneToOneTests_InDetailAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "in details", phraseSetCorrectionsTestLinter, "in detail")
}

func TestPhraseSetCorrectionsOneToOneTests_InDetailRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "c++ - who can tell me \"*this pointer\" in details?", phraseSetCorrectionsTestLinter, "c++ - who can tell me \"*this pointer\" in detail?")
}

func TestPhraseSetCorrectionsOneToOneTests_InMoreDetailAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "in more details", phraseSetCorrectionsTestLinter, "in more detail")
}

func TestPhraseSetCorrectionsOneToOneTests_InMoreDetailRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Document the interface in more details · Issue #3 · owlbarn ...", phraseSetCorrectionsTestLinter, "Document the interface in more detail · Issue #3 · owlbarn ...")
}

func TestPhraseSetCorrectionsOneToOneTests_FixInThisRegards(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am testing many apps for our custom TROMjaro Linux, so I can be helpful in this regards.", phraseSetCorrectionsTestLinter, "I am testing many apps for our custom TROMjaro Linux, so I can be helpful in this regard.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixInThatRegards(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Looks like that are all settings I can make in the Buderus in that regards.", phraseSetCorrectionsTestLinter, "Looks like that are all settings I can make in the Buderus in that regard.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsInflictionPoint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can also position the infliction point of the curve. By default it's exactly at the center in between the two connecting nodes.", phraseSetCorrectionsTestLinter, "You can also position the inflection point of the curve. By default it's exactly at the center in between the two connecting nodes.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsInflictionPoints(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... find where it touches the other side, and measure the distance. Potentially, I'd only have to do it for \"infliction points\".", phraseSetCorrectionsTestLinter, "... find where it touches the other side, and measure the distance. Potentially, I'd only have to do it for \"inflection points\".")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsInvestInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "which represents the amount of money they want to invest into a particular deal.", phraseSetCorrectionsTestLinter, "which represents the amount of money they want to invest in a particular deal.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsInvestingInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Taking dividends in cash (rather than automatically re-investing into the originating fund) can help alleviate the need for rebalancing.", phraseSetCorrectionsTestLinter, "Taking dividends in cash (rather than automatically re-investing in the originating fund) can help alleviate the need for rebalancing.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsInvestedInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it's all automatically invested into a collection of loans that match the criteria that ...", phraseSetCorrectionsTestLinter, "it's all automatically invested in a collection of loans that match the criteria that ...")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsInvestsInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If a user invests into the protocol first using USDC but afterward changing to DAI, ...", phraseSetCorrectionsTestLinter, "If a user invests in the protocol first using USDC but afterward changing to DAI, ...")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsInvestmentInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A $10,000 investment into the fund made on February 28, 1997 would have grown to a value of $42,650 at the end of the 20-year period.", phraseSetCorrectionsTestLinter, "A $10,000 investment in the fund made on February 28, 1997 would have grown to a value of $42,650 at the end of the 20-year period.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsLayouted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "only the views that neeed it will be measured and layouted when the superview changes", phraseSetCorrectionsTestLinter, "only the views that neeed it will be measured and laid out when the superview changes")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsLayouting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "An R package for layouting tables, using the S4 method", phraseSetCorrectionsTestLinter, "An R package for laying out tables, using the S4 method")
}

func TestPhraseSetCorrectionsOneToOneTests_LitotesNotUncommonAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "not uncommon", phraseSetCorrectionsTestLinter, "common")
}

func TestPhraseSetCorrectionsOneToOneTests_LitotesNotUncommonSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is not uncommon to see outages during storms.", phraseSetCorrectionsTestLinter, "It is common to see outages during storms.")
}

func TestPhraseSetCorrectionsOneToOneTests_LitotesNotUnlikely(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This outcome is not unlikely given the data.", phraseSetCorrectionsTestLinter, "This outcome is likely given the data.")
}

func TestPhraseSetCorrectionsOneToOneTests_LitotesNotInsignificant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That is not insignificant progress.", phraseSetCorrectionsTestLinter, "That is significant progress.")
}

func TestPhraseSetCorrectionsOneToOneTests_LitotesMorePreferable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is it more preferable to use process.env.variable or env.parsed.variable?", phraseSetCorrectionsTestLinter, "Is it preferable to use process.env.variable or env.parsed.variable?")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsMakeDueWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For now, I can make due with a bash script I have", phraseSetCorrectionsTestLinter, "For now, I can make do with a bash script I have")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsMadeDueWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I made due with using actions.push for now but will try to do a codepen soon", phraseSetCorrectionsTestLinter, "I made do with using actions.push for now but will try to do a codepen soon")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsMakesDueWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but the code makes due with what is available", phraseSetCorrectionsTestLinter, "but the code makes do with what is available")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsMakingDueWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've been making due with the testMultiple script I wrote above.", phraseSetCorrectionsTestLinter, "I've been making do with the testMultiple script I wrote above.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixMakeSenses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "some symbols make senses only if you have a certain keyboard", phraseSetCorrectionsTestLinter, "some symbols make sense only if you have a certain keyboard")
}

func TestPhraseSetCorrectionsOneToOneTests_FixMadeSenses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Usually on the examples of matlab central I have found all with positive magnitude and made senses to me.", phraseSetCorrectionsTestLinter, "Usually on the examples of matlab central I have found all with positive magnitude and made sense to me.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixMakesSenses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If it makes senses I can open a PR.", phraseSetCorrectionsTestLinter, "If it makes sense I can open a PR.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixMakingSenses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I appreciate you mentioned the two use cases, which are making senses for both.", phraseSetCorrectionsTestLinter, "I appreciate you mentioned the two use cases, which are making sense for both.")
}

func TestPhraseSetCorrectionsOneToOneTests_PointIsMoot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your point is mute.", phraseSetCorrectionsTestLinter, "Your point is moot.")
}

func TestPhraseSetCorrectionsOneToOneTests_OperativeSystem(t *testing.T) {
	linttest.AssertSuggestionResult(t, "COS is a operative system made with the COSMOS Kernel and written in C#, COS its literally the same than MS-DOS but written in C# and open-source.", phraseSetCorrectionsTestLinter, "COS is a operating system made with the COSMOS Kernel and written in C#, COS its literally the same than MS-DOS but written in C# and open-source.")
}

func TestPhraseSetCorrectionsOneToOneTests_OperativeSystems(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My dotfiles for my operative systems and other configurations.", phraseSetCorrectionsTestLinter, "My dotfiles for my operating systems and other configurations.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectPasserbys(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For any passerbys, you may replace visibility: hidden/collapsed with: opacity: 0; pointer-events: none;.", phraseSetCorrectionsTestLinter, "For any passersby, you may replace visibility: hidden/collapsed with: opacity: 0; pointer-events: none;.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectPasserBysHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is there any way for random willing passer-bys to help with this effort?", phraseSetCorrectionsTestLinter, "Is there any way for random willing passers-by to help with this effort?")
}

func TestPhraseSetCorrectionsOneToOneTests_FixPeak(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Offer a peak behind the curtain of what I look for when baselining a software installation.", phraseSetCorrectionsTestLinter, "Offer a peek behind the curtain of what I look for when baselining a software installation.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixPeaked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I peaked behind the curtain of the new Autodraw tool and noticed some expected similarities to what I saw in Quickdraw.", phraseSetCorrectionsTestLinter, "I peeked behind the curtain of the new Autodraw tool and noticed some expected similarities to what I saw in Quickdraw.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixPeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I can see how peaking behind the curtain got me to where I am today.", phraseSetCorrectionsTestLinter, "I can see how peeking behind the curtain got me to where I am today.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixPeaks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Daily Vlog Series that peaks behind the curtain of an Entrepreneur's day to day life in 2016 building a business.", phraseSetCorrectionsTestLinter, "The Daily Vlog Series that peeks behind the curtain of an Entrepreneur's day to day life in 2016 building a business.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixProvocate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hardcoded chainId can provocate a possible replay attacks between chains in the event of a future chain split.", phraseSetCorrectionsTestLinter, "Hardcoded chainId can provoke a possible replay attacks between chains in the event of a future chain split.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixProvocated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "tempering with inconsistent content lengths provocated the error which lead me to encoding?", phraseSetCorrectionsTestLinter, "tempering with inconsistent content lengths provoked the error which lead me to encoding?")
}

func TestPhraseSetCorrectionsOneToOneTests_FixProvocates(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it wont mark the check on the solving square, it provocates a timeout and return 500 Server Error", phraseSetCorrectionsTestLinter, "it wont mark the check on the solving square, it provokes a timeout and return 500 Server Error")
}

func TestPhraseSetCorrectionsOneToOneTests_FixProvocating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "could return incorrect balances provocating an incorrect calculation of rsETH price", phraseSetCorrectionsTestLinter, "could return incorrect balances provoking an incorrect calculation of rsETH price")
}

func TestPhraseSetCorrectionsOneToOneTests_RedundantMoreOptimal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is this more optimal?", phraseSetCorrectionsTestLinter, "Is this optimal?")
}

func TestPhraseSetCorrectionsOneToOneTests_RedundantMostIdeal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is the most ideal scenario.", phraseSetCorrectionsTestLinter, "This is the ideal scenario.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is anyone wanting to step up and take responsibility of this library, or should I put it in EOL and redirect to another tool? ", phraseSetCorrectionsTestLinter, "Is anyone wanting to step up and take responsibility for this library, or should I put it in EOL and redirect to another tool? ")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTaken(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if it had only taken responsibility of the manifest/info additions and extensionsID it would have made our life easier", phraseSetCorrectionsTestLinter, "if it had only taken responsibility for the manifest/info additions and extensionsID it would have made our life easier")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If I have a message that i want to encode, who takes responsibility of pointers?", phraseSetCorrectionsTestLinter, "If I have a message that i want to encode, who takes responsibility for pointers?")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This issue is about taking responsibility of the feature area auto indentation and start solving the bugs in the feature area.", phraseSetCorrectionsTestLinter, "This issue is about taking responsibility for the feature area auto indentation and start solving the bugs in the feature area.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If the driver took responsibility of the locking, it could let these HTTP calls happen in parallel", phraseSetCorrectionsTestLinter, "If the driver took responsibility for the locking, it could let these HTTP calls happen in parallel")
}

func TestPhraseSetCorrectionsOneToOneTests_FixAssume(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it's a relatively big chunk of behavior to assume responsibility of", phraseSetCorrectionsTestLinter, "it's a relatively big chunk of behavior to assume responsibility for")
}

func TestPhraseSetCorrectionsOneToOneTests_FixAssumed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and assumed responsibility of project managing the transition of Barclays", phraseSetCorrectionsTestLinter, "and assumed responsibility for project managing the transition of Barclays")
}

func TestPhraseSetCorrectionsOneToOneTests_FixAssumes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It means that the core development team assumes responsibility of the module", phraseSetCorrectionsTestLinter, "It means that the core development team assumes responsibility for the module")
}

func TestPhraseSetCorrectionsOneToOneTests_FixAssuming(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The point of extract is essentially that you're assuming responsibility of maintenance for that version of the formula.", phraseSetCorrectionsTestLinter, "The point of extract is essentially that you're assuming responsibility for maintenance for that version of the formula.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixClaim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so it doesn't need to claim responsibility of the reappearing containers lifecycle", phraseSetCorrectionsTestLinter, "so it doesn't need to claim responsibility for the reappearing containers lifecycle")
}

func TestPhraseSetCorrectionsOneToOneTests_FixClaimed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a group called The Impact Team had claimed responsibility of the data breach", phraseSetCorrectionsTestLinter, "a group called The Impact Team had claimed responsibility for the data breach")
}

func TestPhraseSetCorrectionsOneToOneTests_FixClaiming(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I feel that there should be some other way of claiming responsibility of the promise's continuation.", phraseSetCorrectionsTestLinter, "I feel that there should be some other way of claiming responsibility for the promise's continuation.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixClaims(t *testing.T) {
	linttest.AssertSuggestionResult(t, "yet the Lord claims responsibility of those boundaries", phraseSetCorrectionsTestLinter, "yet the Lord claims responsibility for those boundaries")
}

func TestPhraseSetCorrectionsOneToOneTests_FixAnEscapeGoat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I see too many times the cable and ps thingy being used as an escape goat.", phraseSetCorrectionsTestLinter, "I see too many times the cable and ps thingy being used as a scapegoat.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixEscapeGoat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It helps shift the reason for the failure on to what the manager did not do (making them the escape goat when it fails).", phraseSetCorrectionsTestLinter, "It helps shift the reason for the failure on to what the manager did not do (making them the scapegoat when it fails).")
}

func TestPhraseSetCorrectionsOneToOneTests_FixEscapeGoats(t *testing.T) {
	linttest.AssertSuggestionResult(t, "People might be using Americans as escape goats for this, but these mishearings are becoming as common as a bowl in a china shop!", phraseSetCorrectionsTestLinter, "People might be using Americans as scapegoats for this, but these mishearings are becoming as common as a bowl in a china shop!")
}

func TestPhraseSetCorrectionsOneToOneTests_FixSeamToBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "amdvlk is deprecated but my system still uses it as default and I can't seam to be able to change it.", phraseSetCorrectionsTestLinter, "amdvlk is deprecated but my system still uses it as default and I can't seem to be able to change it.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixISeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so now whatever i seam to try it doesnt work", phraseSetCorrectionsTestLinter, "so now whatever i seem to try it doesnt work")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWeSeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "using a 4G network we seam to get ICE messages mixing Ipv6 and Ipv4", phraseSetCorrectionsTestLinter, "using a 4G network we seem to get ICE messages mixing Ipv6 and Ipv4")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWeAllSeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if it is your own nation then we all seam to get the update", phraseSetCorrectionsTestLinter, "if it is your own nation then we all seem to get the update")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWeBothSeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "because we both seam to have enough for frivolous things", phraseSetCorrectionsTestLinter, "because we both seem to have enough for frivolous things")
}

func TestPhraseSetCorrectionsOneToOneTests_FixYouSeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Assigning you, since you seam to have already made the fix.", phraseSetCorrectionsTestLinter, "Assigning you, since you seem to have already made the fix.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixYouAllSeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That's a good advice which you all seam to agree upon.", phraseSetCorrectionsTestLinter, "That's a good advice which you all seem to agree upon.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixYouBothSeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "since you both seam to like the game", phraseSetCorrectionsTestLinter, "since you both seem to like the game")
}

func TestPhraseSetCorrectionsOneToOneTests_FixHeSeams(t *testing.T) {
	linttest.AssertSuggestionResult(t, "tagging @PedroTroller as he seams to still be active on this project.", phraseSetCorrectionsTestLinter, "tagging @PedroTroller as he seems to still be active on this project.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixSheSeams(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here is the exact timestamp where she seams to talk about exactly this -> video.", phraseSetCorrectionsTestLinter, "Here is the exact timestamp where she seems to talk about exactly this -> video.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixItSeams(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It seams i cannot use $tries and $timeout properties on my queued listener class?", phraseSetCorrectionsTestLinter, "It seems i cannot use $tries and $timeout properties on my queued listener class?")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTheySeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lets start with the \"not\" and \"and\" gates because they seam the easiest.", phraseSetCorrectionsTestLinter, "Lets start with the \"not\" and \"and\" gates because they seem the easiest.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTheyAllSeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have tried the sum, product, max and min functions and they all seam to work.", phraseSetCorrectionsTestLinter, "I have tried the sum, product, max and min functions and they all seem to work.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixTheyBothSeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's probably cause they both seam to combine martial arts with animal instincts", phraseSetCorrectionsTestLinter, "It's probably cause they both seem to combine martial arts with animal instincts")
}

func TestPhraseSetCorrectionsOneToOneTests_FixEverythingSeams(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Note that if you try to slider the slider first to the right and then to the left, everything seams alright.", phraseSetCorrectionsTestLinter, "Note that if you try to slider the slider first to the right and then to the left, everything seems alright.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixEverybodySeams(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm currently a little disappointed because everybody seams to care only about the Rails framework", phraseSetCorrectionsTestLinter, "I'm currently a little disappointed because everybody seems to care only about the Rails framework")
}

func TestPhraseSetCorrectionsOneToOneTests_FixEveryoneSeams(t *testing.T) {
	linttest.AssertSuggestionResult(t, "everyone seams to use the editor now a days plus there is a tun of extensions available", phraseSetCorrectionsSingleLint("SeamToSeem"), "everyone seems to use the editor now a days plus there is a tun of extensions available")
}

func TestPhraseSetCorrectionsOneToOneTests_FixEveryoneSeamsCombinedWithNowADays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "everyone seams to use the editor now a days plus there is a tun of extensions available", phraseSetCorrectionsTestLinter, "everyone seems to use the editor nowadays plus there is a tun of extensions available")
}

func TestPhraseSetCorrectionsOneToOneTests_FixSettingUpABadExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Are our neighbours setting up a bad example ?", phraseSetCorrectionsTestLinter, "Are our neighbours setting a bad example ?")
}

func TestPhraseSetCorrectionsOneToOneTests_FixSetUpABadExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It works sometimes but set up a bad example.", phraseSetCorrectionsTestLinter, "It works sometimes but set a bad example.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixSetsUpABadExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This sets up a bad example that will make it into a trend. As long as AI is not deterministic, you should review the code.", phraseSetCorrectionsTestLinter, "This sets a bad example that will make it into a trend. As long as AI is not deterministic, you should review the code.")
}

func TestPhraseSetCorrectionsOneToOneTests_SkyRockeeted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i can hear the audio but the firefox disk usage sky rocketed after that", phraseSetCorrectionsTestLinter, "i can hear the audio but the firefox disk usage skyrocketed after that")
}

func TestPhraseSetCorrectionsOneToOneTests_SkyRocketing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "With the rising abilities of agents, expectations are sky rocketing in terms of productivity.", phraseSetCorrectionsTestLinter, "With the rising abilities of agents, expectations are skyrocketing in terms of productivity.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixSubjectivePronoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Gets the subjective pronoun with \"are\"/\"is\" contraction.", phraseSetCorrectionsTestLinter, "Gets the subject pronoun with \"are\"/\"is\" contraction.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixObjectivePronoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If my subjective pronoun is he , then this might be macro expanded as he entered the room", phraseSetCorrectionsTestLinter, "If my subject pronoun is he , then this might be macro expanded as he entered the room")
}

func TestPhraseSetCorrectionsOneToOneTests_FixObjectivePronounTitleCase(t *testing.T) {
	t.Skip("Fails due to #3741")
	linttest.AssertSuggestionResult(t, "Objective Pronouns Examples of Subjects and Objects in a Sentence", phraseSetCorrectionsTestLinter, "Object Pronouns Examples of Subjects and Objects in a Sentence")
}

func TestPhraseSetCorrectionsOneToOneTests_FixSubjectiveAndObjectivePronouns(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To address these questions, we analyze subjective pronouns (he, she) and objective pronouns (him, her).", phraseSetCorrectionsTestLinter, "To address these questions, we analyze subject pronouns (he, she) and object pronouns (him, her).")
}

func TestPhraseSetCorrectionsOneToOneTests_IfOnlyThereWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if only there was an endpoint do to so", phraseSetCorrectionsTestLinter, "if only there were an endpoint do to so")
}

func TestPhraseSetCorrectionsOneToOneTests_IfOnlyIWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Oh If only I was that clever !!", phraseSetCorrectionsTestLinter, "Oh If only I were that clever !!")
}

func TestPhraseSetCorrectionsOneToOneTests_IfOnlyHeWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If only he was kind enough to attempt to contact me in private first", phraseSetCorrectionsTestLinter, "If only he were kind enough to attempt to contact me in private first")
}

func TestPhraseSetCorrectionsOneToOneTests_IfOnlySheWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If only she was right.", phraseSetCorrectionsTestLinter, "If only she were right.")
}

func TestPhraseSetCorrectionsOneToOneTests_IfOnlyItWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if only it was accessible via USB connection - hint hint", phraseSetCorrectionsTestLinter, "if only it were accessible via USB connection - hint hint")
}

func TestPhraseSetCorrectionsOneToOneTests_IWishThereWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wish there was a keyboard shortcut or something that was \"bring back the suggestion you just made in the last 3 seconds\".", phraseSetCorrectionsTestLinter, "I wish there were a keyboard shortcut or something that was \"bring back the suggestion you just made in the last 3 seconds\".")
}

func TestPhraseSetCorrectionsOneToOneTests_IWishIWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wish I was as smart as I think I am.", phraseSetCorrectionsTestLinter, "I wish I were as smart as I think I am.")
}

func TestPhraseSetCorrectionsOneToOneTests_IWishHeWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However I wish he was that smart about ARM chips present in the current mobile devices.", phraseSetCorrectionsTestLinter, "However I wish he were that smart about ARM chips present in the current mobile devices.")
}

func TestPhraseSetCorrectionsOneToOneTests_IWishSheWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wish she was more accepting of her own interests.", phraseSetCorrectionsTestLinter, "I wish she were more accepting of her own interests.")
}

func TestPhraseSetCorrectionsOneToOneTests_IWishItWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but I wish it was more friendly to existing ecosystems", phraseSetCorrectionsTestLinter, "but I wish it were more friendly to existing ecosystems")
}

func TestPhraseSetCorrectionsOneToOneTests_Take(t *testing.T) {
	linttest.AssertSuggestionResult(t, "allowed .editorconfig to set editor ruler and take control over soft-wrap", phraseSetCorrectionsTestLinter, "allowed .editorconfig to set editor ruler and take control of soft-wrap")
}

func TestPhraseSetCorrectionsOneToOneTests_Taken(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've taken control over the inputValue to be able to render the wanted menu items", phraseSetCorrectionsTestLinter, "I've taken control of the inputValue to be able to render the wanted menu items")
}

func TestPhraseSetCorrectionsOneToOneTests_Takes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "AI takes control over players hand", phraseSetCorrectionsTestLinter, "AI takes control of players hand")
}

func TestPhraseSetCorrectionsOneToOneTests_Taking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this inconsistent behavior is very annoying for taking control over your dependency graph", phraseSetCorrectionsTestLinter, "this inconsistent behavior is very annoying for taking control of your dependency graph")
}

func TestPhraseSetCorrectionsOneToOneTests_Took(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Noted drone was NOT stoping and manually took control over it to stop it.", phraseSetCorrectionsTestLinter, "Noted drone was NOT stoping and manually took control of it to stop it.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsGettingUseTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm getting use to it slowly.", phraseSetCorrectionsTestLinter, "I'm getting used to it slowly.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsAreUseTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you are use to Ubuntu, then the way sudo works should not be strange.", phraseSetCorrectionsTestLinter, "If you are used to Ubuntu, then the way sudo works should not be strange.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsImUseTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm use to doing a lot of work.", phraseSetCorrectionsTestLinter, "I'm used to doing a lot of work.")
}

func TestPhraseSetCorrectionsOneToOneTests_AllowsUseToAsVerb(t *testing.T) {
	linttest.AssertNoLints(t, "This is the editor I use to write code.", phraseSetCorrectionsTestLinter)
}

func TestPhraseSetCorrectionsOneToOneTests_AllowsUsedTo(t *testing.T) {
	linttest.AssertNoLints(t, "I used to develop with objects in JS.", phraseSetCorrectionsTestLinter)
}

func TestPhraseSetCorrectionsOneToOneTests_FixWreckHavoc(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tables with a \".\" in the name wreck havoc with the system", phraseSetCorrectionsTestLinter, "Tables with a \".\" in the name wreak havoc with the system")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWreckedHavoc(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would have been some weird local configuration of LO that wrecked havoc.", phraseSetCorrectionsTestLinter, "It would have been some weird local configuration of LO that wreaked havoc.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWreckingHavoc(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Multi-line edit is wrecking havoc with indention", phraseSetCorrectionsTestLinter, "Multi-line edit is wreaking havoc with indention")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWrecksHavoc(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Small POC using rust with ptrace that wrecks havoc on msync", phraseSetCorrectionsTestLinter, "Small POC using rust with ptrace that wreaks havoc on msync")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsVerseAgainst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A game of Morra, with 3 different AI you can verse against.", phraseSetCorrectionsTestLinter, "A game of Morra, with 3 different AI you can play against.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsVersingAgainst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This will help when you are versing against a particular boss.", phraseSetCorrectionsTestLinter, "This will help when you are playing against a particular boss.")
}

func TestPhraseSetCorrectionsOneToOneTests_CorrectsVerseMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Come verse me in this game.", phraseSetCorrectionsTestLinter, "Come play me in this game.")
}

func TestPhraseSetCorrectionsOneToOneTests_AllowsVersus(t *testing.T) {
	linttest.AssertNoLints(t, "It was red versus blue in the finals.", phraseSetCorrectionsTestLinter)
}

func TestPhraseSetCorrectionsOneToOneTests_FixByWrote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Until one repeats and learns a fact by wrote it is the picture that sustains us.", phraseSetCorrectionsTestLinter, "Until one repeats and learns a fact by rote it is the picture that sustains us.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixByWroteHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This specification may then be translated into a recursive-decent parser almost by-wrote.", phraseSetCorrectionsTestLinter, "This specification may then be translated into a recursive-decent parser almost by-rote.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWroteLearning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I found that what turned me off math class was that teachers encouraged wrote learning instead of understanding.", phraseSetCorrectionsTestLinter, "I found that what turned me off math class was that teachers encouraged rote learning instead of understanding.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWroteMemorisation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not much of a wrote memorisation kind of guy, so I preferred to commit them to memory by framing them in the context of a paragraph.", phraseSetCorrectionsTestLinter, "Not much of a rote memorisation kind of guy, so I preferred to commit them to memory by framing them in the context of a paragraph.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWroteMemorisationHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I find it helps me retain information much better and for longer compared to when I just blindly did wrote-memorisation.", phraseSetCorrectionsTestLinter, "I find it helps me retain information much better and for longer compared to when I just blindly did rote-memorisation.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWroteMemorization(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Outside websites are also no-go, exacerbating the need for wrote memorization.", phraseSetCorrectionsTestLinter, "Outside websites are also no-go, exacerbating the need for rote memorization.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWroteMemorizationHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The voicings was the biggest game-changer for me, coming from a wrote-memorization type classical piano background.", phraseSetCorrectionsTestLinter, "The voicings was the biggest game-changer for me, coming from a rote-memorization type classical piano background.")
}

func TestPhraseSetCorrectionsOneToOneTests_FixWroteMemorizing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have never been good at wrote memorizing abbreviations, initialisms, or acronyms.", phraseSetCorrectionsTestLinter, "I have never been good at rote memorizing abbreviations, initialisms, or acronyms.")
}
