package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOrthographicConsistency_DontCapitalizeCommonWordsThatMatchAcronyms(t *testing.T) {
	linttest.AssertNoLints(t, "They told us who it was.", NewOrthographicConsistency())
}

func TestOrthographicConsistency_NasaShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nasa is a governmental institution.", NewOrthographicConsistency(), "NASA is a governmental institution.")
}

func TestOrthographicConsistency_IkeaShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ikea operates a vast retail network.", NewOrthographicConsistency(), "IKEA operates a vast retail network.")
}

func TestOrthographicConsistency_NatoShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nato is a military alliance.", NewOrthographicConsistency(), "NATO is a military alliance.")
}

func TestOrthographicConsistency_FbiShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Fbi investigates federal crimes.", NewOrthographicConsistency(), "FBI investigates federal crimes.")
}

func TestOrthographicConsistency_CiaShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cia gathers intelligence.", NewOrthographicConsistency(), "CIA gathers intelligence.")
}

func TestOrthographicConsistency_HivShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hiv is a virus.", NewOrthographicConsistency(), "HIV is a virus.")
}

func TestOrthographicConsistency_DnaShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Dna carries genetic information.", NewOrthographicConsistency(), "DNA carries genetic information.")
}

func TestOrthographicConsistency_RnaShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rna participates in protein synthesis.", NewOrthographicConsistency(), "RNA participates in protein synthesis.")
}

func TestOrthographicConsistency_CpuShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cpu executes instructions.", NewOrthographicConsistency(), "CPU executes instructions.")
}

func TestOrthographicConsistency_GpuShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Gpu accelerates graphics.", NewOrthographicConsistency(), "GPU accelerates graphics.")
}

func TestOrthographicConsistency_HtmlShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Html structures web documents.", NewOrthographicConsistency(), "HTML structures web documents.")
}

func TestOrthographicConsistency_UrlShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Url identifies a resource.", NewOrthographicConsistency(), "URL identifies a resource.")
}

func TestOrthographicConsistency_FaqShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Faq answers common questions.", NewOrthographicConsistency(), "FAQ answers common questions.")
}

func TestOrthographicConsistency_LinkedinShouldUseCanonicalCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I updated my linkedin profile yesterday.", NewOrthographicConsistency(), "I updated my LinkedIn profile yesterday.")
}

func TestOrthographicConsistency_WordpressShouldUseCanonicalCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She writes daily on her wordpress blog.", NewOrthographicConsistency(), "She writes daily on her WordPress blog.")
}

func TestOrthographicConsistency_IphoneShouldUseCanonicalCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She bought a new iphone last week.", NewOrthographicConsistency(), "She bought a new iPhone last week.")
}

func TestOrthographicConsistency_IpadShouldUseCanonicalCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He reads on his ipad every morning.", NewOrthographicConsistency(), "He reads on his iPad every morning.")
}

func TestOrthographicConsistency_IpodShouldUseCanonicalCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She still listens to her ipod on long flights.", NewOrthographicConsistency(), "She still listens to her iPod on long flights.")
}

func TestOrthographicConsistency_ImacShouldUseCanonicalCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She edits photos on her imac at home.", NewOrthographicConsistency(), "She edits photos on her iMac at home.")
}

func TestOrthographicConsistency_ItunesShouldUseCanonicalCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She bought the album on itunes years ago.", NewOrthographicConsistency(), "She bought the album on iTunes years ago.")
}

func TestOrthographicConsistency_PdfShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Pdf preserves formatting.", NewOrthographicConsistency(), "PDF preserves formatting.")
}

func TestOrthographicConsistency_CeoShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our Ceo approved the budget.", NewOrthographicConsistency(), "Our CEO approved the budget.")
}

func TestOrthographicConsistency_CfoShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Cfo presented the report.", NewOrthographicConsistency(), "The CFO presented the report.")
}

func TestOrthographicConsistency_HrShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Hr team scheduled interviews.", NewOrthographicConsistency(), "The HR team scheduled interviews.")
}

func TestOrthographicConsistency_AiShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ai enables new capabilities.", NewOrthographicConsistency(), "AI enables new capabilities.")
}

func TestOrthographicConsistency_UfoShouldBeAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ufo sightings provoke debate.", NewOrthographicConsistency(), "UFO sightings provoke debate.")
}

func TestOrthographicConsistency_MarkdownShouldBeCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I adore markdown.", NewOrthographicConsistency(), "I adore Markdown.")
}

func TestOrthographicConsistency_CanonicalFormsShouldNotBeFlagged(t *testing.T) {
	sentences := []string{
		"NASA is a governmental institution.",
		"IKEA operates a vast retail network.",
		"LEGO bricks encourage creativity.",
		"NATO is a military alliance.",
		"FBI investigates federal crimes.",
		"CIA gathers intelligence.",
		"HIV is a virus.",
		"DNA carries genetic information.",
		"RNA participates in protein synthesis.",
		"CPU executes instructions.",
		"GPU accelerates graphics.",
		"HTML structures web documents.",
		"URL identifies a resource.",
		"FAQ answers common questions.",
		"I updated my LinkedIn profile yesterday.",
		"She writes daily on her WordPress blog.",
		"PDF preserves formatting.",
		"Our CEO approved the budget.",
		"The CFO presented the report.",
		"The HR team scheduled interviews.",
		"AI enables new capabilities.",
		"UFO sightings provoke debate.",
		"I adore Markdown.",
	}
	for _, sentence := range sentences {
		linttest.AssertNoLints(t, sentence, NewOrthographicConsistency())
	}
}

func TestOrthographicConsistency_CorrectsAccidentalDoubleInitialCapitals(t *testing.T) {
	for _, pair := range [][2]string{
		{"THe", "The"}, {"LIzard", "Lizard"}, {"DOor", "Door"},
		{"THe lizard opened the door.", "The lizard opened the door."},
		{"A LIzard opened the door.", "A Lizard opened the door."},
		{"Open the DOor.", "Open the Door."}, {"\"THe\"", "\"The\""},
		{"LIzard!", "Lizard!"}, {"DOor?", "Door?"}, {"THe, DOor", "The, Door"},
		{"THe\nLIzard", "The\nLizard"}, {"THe\tDOor", "The\tDoor"},
		{"HEllo", "Hello"}, {"WOrld", "World"}, {"ENgland", "England"},
	} {
		linttest.AssertSuggestionResult(t, pair[0], NewOrthographicConsistency(), pair[1])
	}
}

func TestOrthographicConsistency_PreservesOtherCasingAndUnknownWords(t *testing.T) {
	for _, input := range []string{
		"The lizard opened the door.", "Lizard Door", "THE LIZARD DOOR", "NASA",
		"iPhone", "LinkedIn", "WordPress", "macOS", "YouTube", "DO", "PR",
		"QZxqzq", "tHe", "lIzard", "DOOr", "THe-like", "pre-DOor",
	} {
		linttest.AssertNoLints(t, input, NewOrthographicConsistency())
	}
}

func TestOrthographicConsistency_AllowsNews(t *testing.T) {
	linttest.AssertNoLints(t, "This is the best part of the news broadcast.", NewOrthographicConsistency())
}

func TestOrthographicConsistency_AllowsIssue2465(t *testing.T) {
	linttest.AssertNoLints(t, "The post’s problem was not in its complexity.", NewOrthographicConsistency())
}

func TestOrthographicConsistency_NoImproperSuggestionForMacos(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "MacOS", NewOrthographicConsistency(), []string{"macOS"}, []string{"MacOS"})
}

func TestOrthographicConsistency_AcceptCaseVariants(t *testing.T) {
	linttest.AssertNoLints(t, "Pr PR", NewOrthographicConsistency())
	linttest.AssertNoLints(t, "Apr APR", NewOrthographicConsistency())
	linttest.AssertNoLints(t, "install.md", NewOrthographicConsistency())
}

func TestOrthographicConsistency_DontAcceptUndefinedCaseVariants(t *testing.T) {
	t.Skip("Upstream ignored: Words with multiple known orthographic variants are not currently supported")
	linttest.AssertLintCount(t, "pr", NewOrthographicConsistency(), 1)
}

func TestOrthographicConsistency_LetsConfusionDescription(t *testing.T) {
	linttest.AssertNoLints(t, "It's often hard to determine where the subject should go with the word `let`. This rule attempts to find common errors with redundancy and contractions that may lead to confusion for readers.", NewOrthographicConsistency())
}
