package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNounVerbConfusion_CorrectsGoodAdvise(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Good advise", NewNounVerbConfusion(), "Good advice")
}

func TestNounVerbConfusion_CorrectsBadAdvise(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I just wanted to bring attention to this because it stood out to me as potentially bad advise.", NewNounVerbConfusion(), "I just wanted to bring attention to this because it stood out to me as potentially bad advice.")
}

func TestNounVerbConfusion_DontFlagCorrectBetterAdvise(t *testing.T) {
	linttest.AssertLintCount(t, "Hello! I am an engineer at Plexon and am conducting tests with Kilosort4 so we can better advise our clients.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_CorrectBetterAdvise(t *testing.T) {
	t.Skip("'better advise' can be correct as above, or a mistake like here")
	linttest.AssertSuggestionResult(t, "Maybe this will be a decent idea, .or anybody has better advise :)", NewNounVerbConfusion(), "Maybe this will be a decent idea, .or anybody has better advice :)")
}

func TestNounVerbConfusion_DontFlagCorrectBetterBelieve(t *testing.T) {
	linttest.AssertLintCount(t, "You'd better believe this is bbedit-gist-maker.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_CorrectStrongBelieve(t *testing.T) {
	linttest.AssertSuggestionResult(t, "cause my strong believe is that we must give any user to describe whether a post is meant factual", NewNounVerbConfusion(), "cause my strong belief is that we must give any user to describe whether a post is meant factual")
}

func TestNounVerbConfusion_CorrectDeepBreathe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Take deep breathe and Do it again!", NewNounVerbConfusion(), "Take deep breath and Do it again!")
}

func TestNounVerbConfusion_CorrectBadIntend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What do you do if you only see slightly longer posts that may still be acceptable (and not bad intend from the poster)", NewNounVerbConfusion(), "What do you do if you only see slightly longer posts that may still be acceptable (and not bad intent from the poster)")
}

func TestNounVerbConfusion_CorrectsBeliefInsteadOfVerb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I belief in you.", NewNounVerbConfusion(), "I believe in you.")
}

func TestNounVerbConfusion_CorrectsBreathInsteadOfVerb(t *testing.T) {
	t.Skip("`to` can't disambiguate since it's valid between verbs and nouns")
	linttest.AssertSuggestionResult(t, "Remember to breath deeply.", NewNounVerbConfusion(), "Remember to breathe deeply.")
}

func TestNounVerbConfusion_DoesNotFlagCorrectBelieve(t *testing.T) {
	linttest.AssertLintCount(t, "I believe in you.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_DoesNotFlagCorrectBreath(t *testing.T) {
	linttest.AssertLintCount(t, "Take a deep breath.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_FixWhenIBreathYouBreath(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When I breath, you breath!", NewNounVerbConfusion(), "When I breathe, you breathe!")
}

func TestNounVerbConfusion_FixWeatherClimateAndTheAirWeBreath(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Weather Climate and the Air We Breath", NewNounVerbConfusion(), "Weather Climate and the Air We Breathe")
}

func TestNounVerbConfusion_FixAlwaysBreath(t *testing.T) {
	linttest.AssertSuggestionResult(t, "breathing. remember to always breath.", NewNounVerbConfusion(), "breathing. remember to always breathe.")
}

func TestNounVerbConfusion_FixNeverBreathAWord(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And never breath a word about your loss; If you can force your heart and nerve and sinew.", NewNounVerbConfusion(), "And never breathe a word about your loss; If you can force your heart and nerve and sinew.")
}

func TestNounVerbConfusion_FixBreathForSeconds(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Once turned on, the LED on the TX unit would breath for a few seconds, then go completely dead and not responding to objects in front of the sensors.", NewNounVerbConfusion(), "Once turned on, the LED on the TX unit would breathe for a few seconds, then go completely dead and not responding to objects in front of the sensors.")
}

func TestNounVerbConfusion_FixBreathALittleMoreLife(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... up to 12% more performance, could breath a little more life into systems as old as Sandy Bridge.", NewNounVerbConfusion(), "... up to 12% more performance, could breathe a little more life into systems as old as Sandy Bridge.")
}

func TestNounVerbConfusion_FixTheDiversityWeBreath(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Diversity We Breath: Community Diversity", NewNounVerbConfusion(), "The Diversity We Breathe: Community Diversity")
}

func TestNounVerbConfusion_FixBelief(t *testing.T) {
	linttest.AssertSuggestionResult(t, "While I have no plans to return to aerospace I belief it gives me a unique perspective to many challenges.", NewNounVerbConfusion(), "While I have no plans to return to aerospace I believe it gives me a unique perspective to many challenges.")
}

func TestNounVerbConfusion_FixWeBelief(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In contrast to other vendors in e-mobility, we belief that true transparency is only trustworthy if the entire process ...", NewNounVerbConfusion(), "In contrast to other vendors in e-mobility, we believe that true transparency is only trustworthy if the entire process ...")
}

func TestNounVerbConfusion_FixICanBreath(t *testing.T) {
	t.Skip("`underwater` is a marginal noun so `breath underwater` matches the compound noun test.")
	linttest.AssertSuggestionResult(t, "Steps to reproduce Expected behaviour I can breath underwater.", NewNounVerbConfusion(), "Steps to reproduce Expected behaviour I can breathe underwater.")
}

func TestNounVerbConfusion_FixCapsShouldBreath(t *testing.T) {
	linttest.AssertSuggestionResult(t, "CAPS 1 2 3 4 5 A B C D SHOULD BREATH A BIT MORE ?", NewNounVerbConfusion(), "CAPS 1 2 3 4 5 A B C D SHOULD BREATHE A BIT MORE ?")
}

func TestNounVerbConfusion_FixCanYouAdviceMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can you advice me how to train?", NewNounVerbConfusion(), "Can you advise me how to train?")
}

func TestNounVerbConfusion_FixWeCanAdviceYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Feel free to share more details about your use case, so we can advice you specifically based on your case.", NewNounVerbConfusion(), "Feel free to share more details about your use case, so we can advise you specifically based on your case.")
}

func TestNounVerbConfusion_FixWouldAdviceAgainst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So that I would advice against using a spindle in laser mode.", NewNounVerbConfusion(), "So that I would advise against using a spindle in laser mode.")
}

func TestNounVerbConfusion_FixAdviceToListen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The idea of this applicaton was inspired by Ray Dalio, who always advice to listen to people who know more than us by experience.", NewNounVerbConfusion(), "The idea of this applicaton was inspired by Ray Dalio, who always advise to listen to people who know more than us by experience.")
}

func TestNounVerbConfusion_DontFixAdviceOnThat(t *testing.T) {
	t.Skip("`You` is an object pronoun in this example. `It` is also both subject and object.")
	linttest.AssertLintCount(t, "I don't do table returning functions in my code so can't offer you advice on that.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_FixAdviceToStickWithOpenvscode(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But unless you really need it, I would advice to stick with openvscode as there are nearly the same.", NewNounVerbConfusion(), "But unless you really need it, I would advise to stick with openvscode as there are nearly the same.")
}

func TestNounVerbConfusion_FixAdviceToBackUpOsImage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would advice to back up all OS image before any update, because you could lose something what was working previously.", NewNounVerbConfusion(), "I would advise to back up all OS image before any update, because you could lose something what was working previously.")
}

func TestNounVerbConfusion_FixAdviceToUseMsStore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I know we can always advice to use the MS store to download JASP instead", NewNounVerbConfusion(), "I know we can always advise to use the MS store to download JASP instead")
}

func TestNounVerbConfusion_FixShouldIntentBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Should intent be on the blocklist?", NewNounVerbConfusion(), "Should intent be on the blocklist?")
}

func TestNounVerbConfusion_FixIfYouIntent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you intent to use a 64 bits machine, change line 74", NewNounVerbConfusion(), "If you intend to use a 64 bits machine, change line 74")
}

func TestNounVerbConfusion_FixWhatYouWouldIntentToDo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "May I ask what you would intent to do with such a feature?", NewNounVerbConfusion(), "May I ask what you would intend to do with such a feature?")
}

func TestNounVerbConfusion_DontFlagIntentRecords(t *testing.T) {
	linttest.AssertLintCount(t, "there are always intent records associated to the txns", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_FixDidYouAlwaysIntentTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Did you always intent to fight malware? No.", NewNounVerbConfusion(), "Did you always intend to fight malware? No.")
}

func TestNounVerbConfusion_FixWeRecommendYouCreateANewIssueOnGithubExplainingWhatYouIntentToDo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... we recommend you create a new issue on github explaining what you intent to do.", NewNounVerbConfusion(), "... we recommend you create a new issue on github explaining what you intend to do.")
}

func TestNounVerbConfusion_FixIntentToUseNonImportedSymbol(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There's a warning reported for this code, saying that it may intent to use non-imported symbol", NewNounVerbConfusion(), "There's a warning reported for this code, saying that it may intend to use non-imported symbol")
}

func TestNounVerbConfusion_FixToEmphasisThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This one could be used in a dialog to emphasis the surprise.", NewNounVerbConfusion(), "This one could be used in a dialog to emphasize the surprise.")
}

func TestNounVerbConfusion_AllowToEmphasisAtEnd(t *testing.T) {
	linttest.AssertLintCount(t, "Changes literal underscores to emphasis", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_AllowToIntentAdjective(t *testing.T) {
	linttest.AssertLintCount(t, "Cleanup passing statistics to intent aware iterator", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_FixToAdviceAClass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How to advice a class that have been intercepted by another javaagent", NewNounVerbConfusion(), "How to advise a class that have been intercepted by another javaagent")
}

func TestNounVerbConfusion_FixToBreathSome(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You go to the balcony to breath some fresh air and look down at the things outside.", NewNounVerbConfusion(), "You go to the balcony to breathe some fresh air and look down at the things outside.")
}

func TestNounVerbConfusion_FixToEmphasisA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we'd like to emphasis a few points below", NewNounVerbConfusion(), "we'd like to emphasize a few points below")
}

func TestNounVerbConfusion_FixToAdviceTheir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "People who are managing this situation tend to advice their users to lock+unlock their screen", NewNounVerbConfusion(), "People who are managing this situation tend to advise their users to lock+unlock their screen")
}

func TestNounVerbConfusion_FixPositiveAffectOnSmallBusinesses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The new law had a positive affect on small businesses.", NewNounVerbConfusion(), "The new law had a positive effect on small businesses.")
}

func TestNounVerbConfusion_FixMeasuredTheAffectOfCaffeine(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We measured the affect of caffeine on reaction time.", NewNounVerbConfusion(), "We measured the effect of caffeine on reaction time.")
}

func TestNounVerbConfusion_FixSideAffectsIncludedNausea(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The side affects included nausea and fatigue.", NewNounVerbConfusion(), "The side effects included nausea and fatigue.")
}

func TestNounVerbConfusion_FixCauseAndAffectNotSame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cause and affect are not the same thing.", NewNounVerbConfusion(), "Cause and effect are not the same thing.")
}

func TestNounVerbConfusion_FixChangeWillHaveAnAffectOnRevenue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The change will have an affect on our revenue.", NewNounVerbConfusion(), "The change will have an effect on our revenue.")
}

func TestNounVerbConfusion_FixMedicineTookAffectWithinMinutes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The medicine took affect within minutes.", NewNounVerbConfusion(), "The medicine took effect within minutes.")
}

func TestNounVerbConfusion_FixPolicyWillComeIntoAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The policy will come into affect on October 1.", NewNounVerbConfusion(), "The policy will come into effect on October 1.")
}

func TestNounVerbConfusion_FixRulesAreNowInAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The rules are now in affect.", NewNounVerbConfusion(), "The rules are now in effect.")
}

func TestNounVerbConfusion_FixWithImmediateAffectOfficeClosed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "With immediate affect, the office is closed.", NewNounVerbConfusion(), "With immediate effect, the office is closed.")
}

func TestNounVerbConfusion_FixStunningSpecialAffects(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The director used stunning special affects.", NewNounVerbConfusion(), "The director used stunning special effects.")
}

func TestNounVerbConfusion_FixPlaceboAffectCanBePowerful(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The placebo affect can be powerful.", NewNounVerbConfusion(), "The placebo effect can be powerful.")
}

func TestNounVerbConfusion_FixRippleAffectAcrossMarket(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We felt the ripple affect across the entire market.", NewNounVerbConfusion(), "We felt the ripple effect across the entire market.")
}

func TestNounVerbConfusion_FixSnowballAffectAmplifiedProblem(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The snowball affect amplified the problem.", NewNounVerbConfusion(), "The snowball effect amplified the problem.")
}

func TestNounVerbConfusion_FixKnockOnAffectThroughoutTeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That decision had a knock-on affect throughout the team.", NewNounVerbConfusion(), "That decision had a knock-on effect throughout the team.")
}

func TestNounVerbConfusion_FixGreenhouseAffectWarmsPlanet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The greenhouse affect warms the planet.", NewNounVerbConfusion(), "The greenhouse effect warms the planet.")
}

func TestNounVerbConfusion_FixApologyHadLittleAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Her apology had little affect.", NewNounVerbConfusion(), "Her apology had little effect.")
}

func TestNounVerbConfusion_FixSettingsGoIntoAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The new settings go into affect after a restart.", NewNounVerbConfusion(), "The new settings go into effect after a restart.")
}

func TestNounVerbConfusion_FixPutPlanIntoAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They put the new plan into affect last week.", NewNounVerbConfusion(), "They put the new plan into effect last week.")
}

func TestNounVerbConfusion_FixContractComesIntoAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The contract comes into affect at midnight.", NewNounVerbConfusion(), "The contract comes into effect at midnight.")
}

func TestNounVerbConfusion_FixWarningHadNoAffectOnBehavior(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The warning had no affect on his behavior.", NewNounVerbConfusion(), "The warning had no effect on his behavior.")
}

func TestNounVerbConfusion_FixInflationHadOppositeAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Inflation had the opposite affect than expected.", NewNounVerbConfusion(), "Inflation had the opposite effect than expected.")
}

func TestNounVerbConfusion_FixRegulationRemainsInAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The regulation remains in affect until further notice.", NewNounVerbConfusion(), "The regulation remains in effect until further notice.")
}

func TestNounVerbConfusion_FixAppChangesTakeAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The app changes take affect next week.", NewNounVerbConfusion(), "The app changes take effect next week.")
}

func TestNounVerbConfusion_FixSoundAffectsWereAdded(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sound affects were added in post.", NewNounVerbConfusion(), "Sound effects were added in post.")
}

func TestNounVerbConfusion_CorrectsNounSubjectEffectsObject(t *testing.T) {
	linttest.AssertSuggestionResult(t, "System outages effect our customers.", NewNounVerbConfusion(), "System outages affect our customers.")
}

func TestNounVerbConfusion_CorrectsEffectsVariant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This policy effects employee morale.", NewNounVerbConfusion(), "This policy affects employee morale.")
}

func TestNounVerbConfusion_IgnoresEffectChangeIdiom(t *testing.T) {
	linttest.AssertLintCount(t, "Leaders work to effect change in their communities.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_IgnoresEffectNounPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "The effect your plan had was dramatic.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_IgnoresEffectAsResultNoun(t *testing.T) {
	linttest.AssertLintCount(t, "The effect was immediate and obvious.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_IgnoresToEffectSubstitutions(t *testing.T) {
	linttest.AssertLintCount(t, "or it may be desired to effect substitutions", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_IgnoresEffectFollowedByOfPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "We measured the effect of caffeine on sleep.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_IgnoresSideEffectsUsage(t *testing.T) {
	linttest.AssertLintCount(t, "Side effects may include mild nausea.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_IgnoresSpecialEffectsPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "She admired the special effects in the film.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_IgnoresEffectInCauseAndEffect(t *testing.T) {
	linttest.AssertLintCount(t, "The diagram explains cause and effect relationships.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_IgnoresEffectsWithPronounSubject(t *testing.T) {
	linttest.AssertLintCount(t, "Those effects were less severe than expected.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_CorrectsTariffEffectImportPrices(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The new tariff will effect import prices next quarter.", NewNounVerbConfusion(), "The new tariff will affect import prices next quarter.")
}

func TestNounVerbConfusion_CorrectsDroughtsEffectCropYields(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Prolonged droughts severely effect crop yields across the valley.", NewNounVerbConfusion(), "Prolonged droughts severely affect crop yields across the valley.")
}

func TestNounVerbConfusion_CorrectsCaffeineEffectSleep(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Caffeine can effect your sleep architecture.", NewNounVerbConfusion(), "Caffeine can affect your sleep architecture.")
}

func TestNounVerbConfusion_CorrectsBugEffectDevices(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The firmware bug doesn't effect older devices.", NewNounVerbConfusion(), "The firmware bug doesn't affect older devices.")
}

func TestNounVerbConfusion_CorrectsSarcasmEffectMorale(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Her sarcasm seemed to effect the team's morale.", NewNounVerbConfusion(), "Her sarcasm seemed to affect the team's morale.")
}

func TestNounVerbConfusion_CorrectsOutageEffectTimeline(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How will this outage effect our deployment timeline?", NewNounVerbConfusion(), "How will this outage affect our deployment timeline?")
}

func TestNounVerbConfusion_CorrectsTemperaturesEffectBattery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cold temperatures drastically effect lithium-ion battery performance.", NewNounVerbConfusion(), "Cold temperatures drastically affect lithium-ion battery performance.")
}

func TestNounVerbConfusion_CorrectsPolicyEffectEligibility(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The policy change could effect your eligibility for benefits.", NewNounVerbConfusion(), "The policy change could affect your eligibility for benefits.")
}

func TestNounVerbConfusion_CorrectsVariablesEffectResults(t *testing.T) {
	linttest.AssertSuggestionResult(t, "These confounding variables may effect the study's results.", NewNounVerbConfusion(), "These confounding variables may affect the study's results.")
}

func TestNounVerbConfusion_CorrectsFansEffectConcentration(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The noisy HVAC fans constantly effect concentration in the lab.", NewNounVerbConfusion(), "The noisy HVAC fans constantly affect concentration in the lab.")
}

func TestNounVerbConfusion_CorrectsHormonesEffectImmunity(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Stress hormones can effect immune response during recovery.", NewNounVerbConfusion(), "Stress hormones can affect immune response during recovery.")
}

func TestNounVerbConfusion_CorrectsPacingEffectEngagement(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The instructor's pacing tended to effect student engagement.", NewNounVerbConfusion(), "The instructor's pacing tended to affect student engagement.")
}

func TestNounVerbConfusion_CorrectsHumidityEffectPaint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Humidity levels directly effect paint curing time.", NewNounVerbConfusion(), "Humidity levels directly affect paint curing time.")
}

func TestNounVerbConfusion_CorrectsExchangeEffectInvoice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The exchange rate will surely effect the final invoice.", NewNounVerbConfusion(), "The exchange rate will surely affect the final invoice.")
}

func TestNounVerbConfusion_CorrectsBrightnessEffectContrast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Screen brightness settings can effect perceived contrast.", NewNounVerbConfusion(), "Screen brightness settings can affect perceived contrast.")
}

func TestNounVerbConfusion_CorrectsMedicationEffectHim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The medication didn't effect him the way the doctor expected.", NewNounVerbConfusion(), "The medication didn't affect him the way the doctor expected.")
}

func TestNounVerbConfusion_CorrectsPaymentsEffectCredit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Late payments will negatively effect your credit score.", NewNounVerbConfusion(), "Late payments will negatively affect your credit score.")
}

func TestNounVerbConfusion_CorrectsWordingEffectInterpretation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Minor wording tweaks shouldn't effect the legal interpretation.", NewNounVerbConfusion(), "Minor wording tweaks shouldn't affect the legal interpretation.")
}

func TestNounVerbConfusion_CorrectsTrafficEffectDelivery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Traffic patterns often effect delivery windows downtown.", NewNounVerbConfusion(), "Traffic patterns often affect delivery windows downtown.")
}

func TestNounVerbConfusion_CorrectsRumorEffectConfidence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The rumor started to effect investor confidence by noon.", NewNounVerbConfusion(), "The rumor started to affect investor confidence by noon.")
}

func TestNounVerbConfusion_CorrectsAllergiesEffectProductivity(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Seasonal allergies badly effect her productivity each April.", NewNounVerbConfusion(), "Seasonal allergies badly affect her productivity each April.")
}

func TestNounVerbConfusion_CorrectsFeedbackEffectRoadmap(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your feedback won't immediately effect the roadmap.", NewNounVerbConfusion(), "Your feedback won't immediately affect the roadmap.")
}

func TestNounVerbConfusion_CorrectsRulesEffectHoneypot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I cant seem to get my additional rules to effect the honeypot", NewNounVerbConfusion(), "I cant seem to get my additional rules to affect the honeypot")
}

func TestNounVerbConfusion_CorrectsBandwidthEffectVideo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Fluctuating bandwidth can effect video call quality.", NewNounVerbConfusion(), "Fluctuating bandwidth can affect video call quality.")
}

func TestNounVerbConfusion_CorrectsGradientEffectSensor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The temperature gradient might effect the sensor's calibration.", NewNounVerbConfusion(), "The temperature gradient might affect the sensor's calibration.")
}

func TestNounVerbConfusion_CorrectsDelaysEffectSatisfaction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Even tiny delays can effect user satisfaction metrics.", NewNounVerbConfusion(), "Even tiny delays can affect user satisfaction metrics.")
}

func TestNounVerbConfusion_CorrectsArchitectureEffectGps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The surrounding architecture can effect GPS accuracy.", NewNounVerbConfusion(), "The surrounding architecture can affect GPS accuracy.")
}

func TestNounVerbConfusion_CorrectsLightingEffectColor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lighting conditions strongly effect color perception.", NewNounVerbConfusion(), "Lighting conditions strongly affect color perception.")
}

func TestNounVerbConfusion_CorrectsCoachEffectRoles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The new coach's strategy will effect players' roles.", NewNounVerbConfusion(), "The new coach's strategy will affect players' roles.")
}

func TestNounVerbConfusion_CorrectsOvertrainingEffectReaction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Overtraining can effect reaction time and coordination.", NewNounVerbConfusion(), "Overtraining can affect reaction time and coordination.")
}

func TestNounVerbConfusion_CorrectsLabelEffectBehavior(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The warning label may effect how consumers use the product.", NewNounVerbConfusion(), "The warning label may affect how consumers use the product.")
}

func TestNounVerbConfusion_CorrectsBecauseAffectIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I worry because affect is hidden.", NewNounVerbConfusion(), "I worry because effect is hidden.")
}

func TestNounVerbConfusion_IgnoresPsychologyUsage(t *testing.T) {
	linttest.AssertLintCount(t, "The patient's affect is flat.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_CorrectsPositiveAffectOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The new law had a positive affect on small businesses.", NewNounVerbConfusion(), "The new law had a positive effect on small businesses.")
}

func TestNounVerbConfusion_CorrectsGreatAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "badges that they provide to users to allow them to promote their projects to great affect", NewNounVerbConfusion(), "badges that they provide to users to allow them to promote their projects to great effect")
}

func TestNounVerbConfusion_CorrectsAffectOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We measured the affect of caffeine on reaction time.", NewNounVerbConfusion(), "We measured the effect of caffeine on reaction time.")
}

func TestNounVerbConfusion_CorrectsSideAffects(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The side affects included nausea and fatigue.", NewNounVerbConfusion(), "The side effects included nausea and fatigue.")
}

func TestNounVerbConfusion_NoChangeSideEffect(t *testing.T) {
	linttest.AssertLintCount(t, "I forgot to test the side effect that users are deleted when clearing data.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_CorrectsCauseAndAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cause and affect are not the same thing.", NewNounVerbConfusion(), "Cause and effect are not the same thing.")
}

func TestNounVerbConfusion_CorrectsHaveAnAffectOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The change will have an affect on our revenue.", NewNounVerbConfusion(), "The change will have an effect on our revenue.")
}

func TestNounVerbConfusion_CorrectsTookAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The medicine took affect within minutes.", NewNounVerbConfusion(), "The medicine took effect within minutes.")
}

func TestNounVerbConfusion_CorrectsComeIntoAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The policy will come into affect on October 1.", NewNounVerbConfusion(), "The policy will come into effect on October 1.")
}

func TestNounVerbConfusion_CorrectsInAffectSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The rules are now in affect.", NewNounVerbConfusion(), "The rules are now in effect.")
}

func TestNounVerbConfusion_CorrectsWithImmediateAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "With immediate affect, the office is closed.", NewNounVerbConfusion(), "With immediate effect, the office is closed.")
}

func TestNounVerbConfusion_CorrectsSpecialAffects(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The director used stunning special affects.", NewNounVerbConfusion(), "The director used stunning special effects.")
}

func TestNounVerbConfusion_CorrectsPlaceboAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The placebo affect can be powerful.", NewNounVerbConfusion(), "The placebo effect can be powerful.")
}

func TestNounVerbConfusion_CorrectsRippleAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We felt the ripple affect across the entire market.", NewNounVerbConfusion(), "We felt the ripple effect across the entire market.")
}

func TestNounVerbConfusion_CorrectsSnowballAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The snowball affect amplified the problem.", NewNounVerbConfusion(), "The snowball effect amplified the problem.")
}

func TestNounVerbConfusion_CorrectsKnockOnAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That decision had a knock-on affect throughout the team.", NewNounVerbConfusion(), "That decision had a knock-on effect throughout the team.")
}

func TestNounVerbConfusion_CorrectsGreenhouseAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The greenhouse affect warms the planet.", NewNounVerbConfusion(), "The greenhouse effect warms the planet.")
}

func TestNounVerbConfusion_CorrectsLittleAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Her apology had little affect.", NewNounVerbConfusion(), "Her apology had little effect.")
}

func TestNounVerbConfusion_CorrectsGoIntoAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The new settings go into affect after a restart.", NewNounVerbConfusion(), "The new settings go into effect after a restart.")
}

func TestNounVerbConfusion_CorrectsPutPlanIntoAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They put the new plan into affect last week.", NewNounVerbConfusion(), "They put the new plan into effect last week.")
}

func TestNounVerbConfusion_CorrectsContractIntoAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The contract comes into affect at midnight.", NewNounVerbConfusion(), "The contract comes into effect at midnight.")
}

func TestNounVerbConfusion_CorrectsNoAffectOnBehavior(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The warning had no affect on his behavior.", NewNounVerbConfusion(), "The warning had no effect on his behavior.")
}

func TestNounVerbConfusion_CorrectsOppositeAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Inflation had the opposite affect than expected.", NewNounVerbConfusion(), "Inflation had the opposite effect than expected.")
}

func TestNounVerbConfusion_CorrectsRemainsInAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The regulation remains in affect until further notice.", NewNounVerbConfusion(), "The regulation remains in effect until further notice.")
}

func TestNounVerbConfusion_CorrectsTakeAffectNextWeek(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The app changes take affect next week.", NewNounVerbConfusion(), "The app changes take effect next week.")
}

func TestNounVerbConfusion_CorrectsSoundAffects(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sound affects were added in post.", NewNounVerbConfusion(), "Sound effects were added in post.")
}

func TestNounVerbConfusion_DoesNotFlagBestAffect(t *testing.T) {
	linttest.AssertLintCount(t, "Using linear regression to predict and understand what factors best affect house price", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_DoesNotFlagSoundAffect(t *testing.T) {
	linttest.AssertLintCount(t, "The goal of this study was to learn what properties of sound affect human focus the most.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_CorrectsSoundAffect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Diesel Generator's animation returns to 'idle' state, but it's sound affect remains in the 'work' state.", NewNounVerbConfusion(), "Diesel Generator's animation returns to 'idle' state, but it's sound effect remains in the 'work' state.")
}

func TestNounVerbConfusion_DoesNotFlagAffectAsVerb(t *testing.T) {
	linttest.AssertLintCount(t, "The change will affect our revenue significantly.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_DoesNotFlagAffectsAsVerb(t *testing.T) {
	linttest.AssertLintCount(t, "This policy directly affects remote workers.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_DoesNotFlagCorrectEffectNoun(t *testing.T) {
	linttest.AssertLintCount(t, "The placebo effect can be powerful.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_DoesNotFlagSoundEffects(t *testing.T) {
	linttest.AssertLintCount(t, "Sound effects were added in post.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_Issue1997(t *testing.T) {
	linttest.AssertNoLints(t, "It depends on which sources it affects, what parameters it uses, etc.", NewNounVerbConfusion())
}

func TestNounVerbConfusion_Issue1996(t *testing.T) {
	linttest.AssertNoLints(t, "Avoid effects outside of functions.", NewNounVerbConfusion())
}

func TestNounVerbConfusion_Issue2008(t *testing.T) {
	linttest.AssertNoLints(t, "Changes that only affect static types, without breaking runtime behavior.", NewNounVerbConfusion())
}

func TestNounVerbConfusion_Issue2041(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let me give you a piece of advise.", NewNounVerbConfusion(), "Let me give you a piece of advice.")
}

func TestNounVerbConfusion_FixHelpsYouWeight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "An iOS app that helps you weight small things on the screen of your iPhone / iPad.", NewNounVerbConfusion(), "An iOS app that helps you weigh small things on the screen of your iPhone / iPad.")
}

func TestNounVerbConfusion_FixDoYouWeight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How much do you weight?", NewNounVerbConfusion(), "How much do you weigh?")
}

func TestNounVerbConfusion_FixMoreThanYouWeight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "contributed more than you weight", NewNounVerbConfusion(), "contributed more than you weigh")
}

func TestNounVerbConfusion_NoFlagSideEffectsFromMedication(t *testing.T) {
	linttest.AssertLintCount(t, "There were no side effects from the medication.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_NoFlagSideEffectsInFunctions(t *testing.T) {
	linttest.AssertLintCount(t, "Avoid side effects in your functions.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_NoFlagThisChangeHasNoSideEffects(t *testing.T) {
	linttest.AssertLintCount(t, "This change has no side effects.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_NoFlagPaddingSideAffectsResults(t *testing.T) {
	linttest.AssertLintCount(t, "I am still not clear how padding side affects the results.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_NoFlagWhatSideAffectsImportMachinery(t *testing.T) {
	linttest.AssertLintCount(t, "Move that to the top level so you don't need to worry about what side affects the import machinery.", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_NoFlagScriptSideAffectOtherSide(t *testing.T) {
	linttest.AssertLintCount(t, "Would an unfreed reference in the script side affect the other side somehow?", NewNounVerbConfusion(), 0)
}

func TestNounVerbConfusion_FixBeforeIDescent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Before I descent into further details, lets understand what are standard and universal Mach-O binaries.", NewNounVerbConfusion(), "Before I descend into further details, lets understand what are standard and universal Mach-O binaries.")
}

func TestNounVerbConfusion_FixButShouldDescent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But we need to calculate how fast we should descent to maintain the glideslope profile.", NewNounVerbConfusion(), "But we need to calculate how fast we should descend to maintain the glideslope profile.")
}

func TestNounVerbConfusion_FixStopADescend(t *testing.T) {
	t.Skip("this context is not yet flagged")
	linttest.AssertSuggestionResult(t, "But in case of low throttle margin you might not be able to stop a descend due to yaw control.", NewNounVerbConfusion(), "But in case of low throttle margin you might not be able to stop a descent due to yaw control.")
}
