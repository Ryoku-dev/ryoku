package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRedundantSelf_FixSelfHarmHerselfHy(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Camille about to self-harm herself", NewRedundantSelf(), []string{
		"Camille about to harm herself",
		"Camille about to self-harm",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmingHerselfHy(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "At times she was suicidal and started self-harming herself when she was 12.", NewRedundantSelf(), []string{
		"At times she was suicidal and started harming herself when she was 12.",
		"At times she was suicidal and started self-harming when she was 12.",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmingHerselfSp(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Violet self harming herself is inter-spliced in this montage and Tate walks in to tell her she's cutting the wrong way.", NewRedundantSelf(), []string{
		"Violet harming herself is inter-spliced in this montage and Tate walks in to tell her she's cutting the wrong way.",
		"Violet self harming is inter-spliced in this montage and Tate walks in to tell her she's cutting the wrong way.",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmingHimselfHy(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I don't know the reason till now for doing so but I'm sure he was self-harming himself.", NewRedundantSelf(), []string{
		"I don't know the reason till now for doing so but I'm sure he was harming himself.",
		"I don't know the reason till now for doing so but I'm sure he was self-harming.",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmingMyselfSp(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I'm no programmer or anything like that but I like staying up late at night and self harming myself trying to get ajax to work.", NewRedundantSelf(), []string{
		"I'm no programmer or anything like that but I like staying up late at night and harming myself trying to get ajax to work.",
		"I'm no programmer or anything like that but I like staying up late at night and self harming trying to get ajax to work.",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmingMyselfHy(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "How I stopped self-harming myself", NewRedundantSelf(), []string{"How I stopped harming myself", "How I stopped self-harming"}, []string{})
}

func TestRedundantSelf_FixSelfHarmToMyselfHy(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I'm glad that I didn't do any self-harm to myself", NewRedundantSelf(), []string{
		"I'm glad that I didn't do any harm to myself",
		"I'm glad that I didn't do any self-harm",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmToMyselfSp(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "so I did self harm to myself with the teacher noticing", NewRedundantSelf(), []string{
		"so I did harm to myself with the teacher noticing",
		"so I did self harm with the teacher noticing",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmedMyselfSp(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I haven't self harmed myself in exactly 3 months and 4 days!!!", NewRedundantSelf(), []string{
		"I haven't harmed myself in exactly 3 months and 4 days!!!",
		"I haven't self harmed in exactly 3 months and 4 days!!!",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmedThemselfSp(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "trusted you enough to tell you that they self harmed themself", NewRedundantSelf(), []string{
		"trusted you enough to tell you that they harmed themself",
		"trusted you enough to tell you that they self harmed",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmingYourselfHy(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Is psychologically self-harming yourself on purpose possible", NewRedundantSelf(), []string{
		"Is psychologically harming yourself on purpose possible",
		"Is psychologically self-harming on purpose possible",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmingYourselfSp(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "What isn't seen as self harming yourself but actually is?", NewRedundantSelf(), []string{
		"What isn't seen as harming yourself but actually is?",
		"What isn't seen as self harming but actually is?",
	}, []string{})
}

func TestRedundantSelf_FixSelfHarmToYourselfHy(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "First of all, do not turn to physical abuse or any type of self-harm to yourself.", NewRedundantSelf(), []string{
		"First of all, do not turn to physical abuse or any type of harm to yourself.",
		"First of all, do not turn to physical abuse or any type of self-harm.",
	}, []string{})
}

func TestRedundantSelf_FixSelfTeachingMyself(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a reference of me self-teaching myself the Barvinok algorithm.", NewRedundantSelf(), "This is a reference of me teaching myself the Barvinok algorithm.")
}

func TestRedundantSelf_FixSelfTaughtMyself(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Since I wasn't blessed to learn security related topic back when I was undergrad, I self taught myself using udemy, and CTF Challenge online.", NewRedundantSelf(), "Since I wasn't blessed to learn security related topic back when I was undergrad, I taught myself using udemy, and CTF Challenge online.")
}

func TestRedundantSelf_FixSelfTeachMyself(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I decided to self-teach myself python so I could get more involved in wider aspects of research, specifically data science.", NewRedundantSelf(), "I decided to teach myself python so I could get more involved in wider aspects of research, specifically data science.")
}

func TestRedundantSelf_FixSelfTeachingThemselves(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The point of OSSU is to make things easier for people self-teaching themselves CS.", NewRedundantSelf(), "The point of OSSU is to make things easier for people teaching themselves CS.")
}

func TestRedundantSelf_FixSelfHostItYourself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "If you want to self-host it yourself, get the latest release and download the distributable zip file attached to it", NewRedundantSelf(), []string{
		"If you want to self-host it, get the latest release and download the distributable zip file attached to it",
		"If you want to host it yourself, get the latest release and download the distributable zip file attached to it",
	}, []string{})
}

func TestRedundantSelf_FixSelfHostingItThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I've open-sourced the codebase and written an installation guide to make it as easy as possible for others who are interested in self-hosting it themselves.", NewRedundantSelf(), []string{
		"I've open-sourced the codebase and written an installation guide to make it as easy as possible for others who are interested in self-hosting it.",
		"I've open-sourced the codebase and written an installation guide to make it as easy as possible for others who are interested in hosting it themselves.",
	}, []string{})
}

func TestRedundantSelf_FixSelfHostedOneMyself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I though WalletConnect bridge server might be too flooded with requests, so I self-hosted one myself, same result.", NewRedundantSelf(), []string{
		"I though WalletConnect bridge server might be too flooded with requests, so I self-hosted one, same result.",
		"I though WalletConnect bridge server might be too flooded with requests, so I hosted one myself, same result.",
	}, []string{})
}

func TestRedundantSelf_FixSelfDiagnosedMyself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "As a professional amateur doctor I have self-diagnosed myself with a mild case of alzheimer", NewRedundantSelf(), []string{
		"As a professional amateur doctor I have self-diagnosed with a mild case of alzheimer",
		"As a professional amateur doctor I have diagnosed myself with a mild case of alzheimer",
	}, []string{})
}

func TestRedundantSelf_FixSelfDiagnoseHimself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "a user can add his X-ray image in the application and he will be able to self diagnose himself", NewRedundantSelf(), []string{
		"a user can add his X-ray image in the application and he will be able to self diagnose",
		"a user can add his X-ray image in the application and he will be able to diagnose himself",
	}, []string{})
}

func TestRedundantSelf_FixSelfDiagnoseItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "a spare should self-diagnose itself and generate then store its performance stats in performance.txt", NewRedundantSelf(), []string{
		"a spare should self-diagnose and generate then store its performance stats in performance.txt",
		"a spare should diagnose itself and generate then store its performance stats in performance.txt",
	}, []string{})
}

func TestRedundantSelf_FixSelfDiagnoseThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Team application designed as a mediatery to help users self diagnose themselves or seek medical attention", NewRedundantSelf(), []string{
		"Team application designed as a mediatery to help users self diagnose or seek medical attention",
		"Team application designed as a mediatery to help users diagnose themselves or seek medical attention",
	}, []string{})
}

func TestRedundantSelf_FixSelfDescribeMyself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Moreover, each thing itself can self-describe itself under well-known, e.g.", NewRedundantSelf(), []string{
		"Moreover, each thing itself can self-describe under well-known, e.g.",
		"Moreover, each thing itself can describe itself under well-known, e.g.",
	}, []string{})
}

func TestRedundantSelf_FixSelfDescribedYourself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I know this is asking for a lot, but since you self-described yourself as a documentation nerd- would you be willing to help ...", NewRedundantSelf(), []string{
		"I know this is asking for a lot, but since you self-described as a documentation nerd- would you be willing to help ...",
		"I know this is asking for a lot, but since you described yourself as a documentation nerd- would you be willing to help ...",
	}, []string{})
}

func TestRedundantSelf_FixSelfDescribesItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "httpx self-describes itself as HTTPX is a fully featured HTTP client for Python 3", NewRedundantSelf(), []string{
		"httpx self-describes as HTTPX is a fully featured HTTP client for Python 3",
		"httpx describes itself as HTTPX is a fully featured HTTP client for Python 3",
	}, []string{})
}

func TestRedundantSelf_FixSelfDescribeThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "This, along with some other capabilities, will better allow route handler delegates to self-describe themselves from just the type", NewRedundantSelf(), []string{
		"This, along with some other capabilities, will better allow route handler delegates to self-describe from just the type",
		"This, along with some other capabilities, will better allow route handler delegates to describe themselves from just the type",
	}, []string{})
}

func TestRedundantSelf_FixSelfMedicateOurselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Needless to say we took advantage of this to self-medicate ourselves during the scheduled downtime.", NewRedundantSelf(), []string{
		"Needless to say we took advantage of this to self-medicate during the scheduled downtime.",
		"Needless to say we took advantage of this to medicate ourselves during the scheduled downtime.",
	}, []string{})
}

func TestRedundantSelf_FixSelfMedicateThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "the patient decides to self-medicate themselves, thereby subjecting themselves to additional harms", NewRedundantSelf(), []string{
		"the patient decides to self-medicate, thereby subjecting themselves to additional harms",
		"the patient decides to medicate themselves, thereby subjecting themselves to additional harms",
	}, []string{})
}

func TestRedundantSelf_FixSelfSabotagingYourself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "How to stop self-sabotaging yourself.  (My struggle with self-sabotagers) has already been recorded in the archive", NewRedundantSelf(), []string{
		"How to stop self-sabotaging.  (My struggle with self-sabotagers) has already been recorded in the archive",
		"How to stop sabotaging yourself.  (My struggle with self-sabotagers) has already been recorded in the archive",
	}, []string{})
}

func TestRedundantSelf_FixSelfSabotageThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I also just don't get why as an artist you'd self sabotage yourself, but you do you if you want you work to be replaced by lazy bastards", NewRedundantSelf(), []string{
		"I also just don't get why as an artist you'd self sabotage, but you do you if you want you work to be replaced by lazy bastards",
		"I also just don't get why as an artist you'd sabotage yourself, but you do you if you want you work to be replaced by lazy bastards",
	}, []string{})
}

func TestRedundantSelf_FixSelfCorrectItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Going against the gradient assure that the algorithm self-correct itself, i.e. we obtain parameters that lead to predictions closer and closer to the actual ...", NewRedundantSelf(), []string{
		"Going against the gradient assure that the algorithm correct itself, i.e. we obtain parameters that lead to predictions closer and closer to the actual ...",
		"Going against the gradient assure that the algorithm self-correct, i.e. we obtain parameters that lead to predictions closer and closer to the actual ...",
	}, []string{})
}

func TestRedundantSelf_FixSelfCorrectsItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Self corrects itself by fetching the width of the parent", NewRedundantSelf(), []string{
		"Self corrects by fetching the width of the parent",
		"Corrects itself by fetching the width of the parent",
	}, []string{})
}

func TestRedundantSelf_FixSelfCorrectingItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "the server handling the http site being out of date and then self-correcting itself", NewRedundantSelf(), []string{
		"the server handling the http site being out of date and then correcting itself",
		"the server handling the http site being out of date and then self-correcting",
	}, []string{})
}

func TestRedundantSelf_FixSelfRegulateMyself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Fun fact: Self Regulate myself with Meditation", NewRedundantSelf(), []string{

		"Fun fact: Self Regulate with Meditation",
	}, []string{})
}

func TestRedundantSelf_FixSelfRegulatingOurselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "in general we should be self regulating ourselves and trying to help each other succeed", NewRedundantSelf(), []string{
		"in general we should be regulating ourselves and trying to help each other succeed",
		"in general we should be self regulating and trying to help each other succeed",
	}, []string{})
}

func TestRedundantSelf_FixSelfCensorThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "this would not be desirable, as capturees would be able to self-censor themselves when captured", NewRedundantSelf(), []string{
		"this would not be desirable, as capturees would be able to censor themselves when captured",
		"this would not be desirable, as capturees would be able to self-censor when captured",
	}, []string{})
}

func TestRedundantSelf_FixSelfCensoringThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Although people often realize that they are self-censoring themselves, it is not always the case.", NewRedundantSelf(), []string{
		"Although people often realize that they are censoring themselves, it is not always the case.",
		"Although people often realize that they are self-censoring, it is not always the case.",
	}, []string{})
}

func TestRedundantSelf_FixSelfGovernThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "system that enables Founders and investors to coordinate and self-govern themselves through a set of self-executing rules published on a public blockchain", NewRedundantSelf(), []string{
		"system that enables Founders and investors to coordinate and govern themselves through a set of self-executing rules published on a public blockchain",
		"system that enables Founders and investors to coordinate and self-govern through a set of self-executing rules published on a public blockchain",
	}, []string{})
}

func TestRedundantSelf_FixSelfIdentifyItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "We need this in order for a job to be able to self-identify itself and potentially send it's own ID to a different service", NewRedundantSelf(), []string{
		"We need this in order for a job to be able to identify itself and potentially send it's own ID to a different service",
		"We need this in order for a job to be able to self-identify and potentially send it's own ID to a different service",
	}, []string{})
}

func TestRedundantSelf_FixSelfIdentifiedThemselves(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "This is how the workers self-identified themselves to the scheduler.", NewRedundantSelf(), []string{
		"This is how the workers identified themselves to the scheduler.",
		"This is how the workers self-identified to the scheduler.",
	}, []string{})
}

func TestRedundantSelf_DontFlagSelfHarmItself(t *testing.T) {
	linttest.AssertNoLints(t, "I understand that self harm itself isn't something that you would have to report.", NewRedundantSelf())
}

func TestRedundantSelf_DontFlagSelfHostWithoutPronoun(t *testing.T) {
	linttest.AssertNoLints(t, "I prefer to self-host my applications", NewRedundantSelf())
}

func TestRedundantSelf_DontFlagHostWithoutSelf(t *testing.T) {
	linttest.AssertNoLints(t, "They decided to host it themselves", NewRedundantSelf())
}

func TestRedundantSelf_FixSelfRightItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Witch Doctor was able to self-right itself but End Game delivered another massive hit that sent Witch Doctor several feet in the air.", NewRedundantSelf(), []string{
		"Witch Doctor was able to right itself but End Game delivered another massive hit that sent Witch Doctor several feet in the air.",
		"Witch Doctor was able to self-right but End Game delivered another massive hit that sent Witch Doctor several feet in the air.",
	}, []string{})
}

func TestRedundantSelf_FixSelfRightsItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Bite Force self-rights itself.", NewRedundantSelf(), []string{"Bite Force rights itself.", "Bite Force self-rights."}, []string{})
}

func TestRedundantSelf_FixSelfRightedItself(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "End Game took Witch Doctor for a ride into the screws but Witch Doctor once again self-righted itself.", NewRedundantSelf(), []string{
		"End Game took Witch Doctor for a ride into the screws but Witch Doctor once again righted itself.",
		"End Game took Witch Doctor for a ride into the screws but Witch Doctor once again self-righted.",
	}, []string{})
}

func TestRedundantSelf_FixSelfDefendItself(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm on dudes side, the person that self defended himself", NewRedundantSelf(), "I'm on dudes side, the person that defended himself")
}

func TestRedundantSelf_FixSelfLevelsItself(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That's going to stop the fuel coming out of this tank and into the other two tanks because the fuel self-levels itself to keep the boat nice and flat.", NewRedundantSelf(), "That's going to stop the fuel coming out of this tank and into the other two tanks because the fuel self-levels to keep the boat nice and flat.")
}
