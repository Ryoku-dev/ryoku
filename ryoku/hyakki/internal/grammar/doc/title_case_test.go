package doc

import (
	"ryoku-rashin/internal/grammar/dict"
	"testing"
)

func TestNormal(t *testing.T) {
	got := MakeTitleCaseStr("this is a test", PlainEnglish{}, dict.Curated())
	want := "This Is a Test"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestComplex(t *testing.T) {
	got := MakeTitleCaseStr("the first and last words should be capitalized, even if it is \"the\"", PlainEnglish{}, dict.Curated())
	want := "The First and Last Words Should Be Capitalized, Even If It Is \"The\""
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestAboutUppercaseWithNumbers(t *testing.T) {
	got := MakeTitleCaseStr("0 about 0", PlainEnglish{}, dict.Curated())
	want := "0 About 0"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestTabABecomesUpcase(t *testing.T) {
	got := MakeTitleCaseStr("\ta", PlainEnglish{}, dict.Curated())
	want := "\tA"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestFixesVideoPress(t *testing.T) {
	got := MakeTitleCaseStr("videopress", PlainEnglish{}, dict.Curated())
	want := "VideoPress"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestPreservesIcloudCamelCaseMidSentence(t *testing.T) {
	got := MakeTitleCaseStr("she backs up photos to icloud", PlainEnglish{}, dict.Curated())
	want := "She Backs up Photos to iCloud"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestUnitedStates(t *testing.T) {
	got := MakeTitleCaseStr("united states", PlainEnglish{}, dict.Curated())
	want := "United States"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestKeepsDecimal(t *testing.T) {
	got := MakeTitleCaseStr("harper turns 1.0 today", PlainEnglish{}, dict.Curated())
	want := "Harper Turns 1.0 Today"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestFixesOddCapitalizedProperNouns(t *testing.T) {
	got := MakeTitleCaseStr("i spoke at wordcamp u.s. in 2025", PlainEnglish{}, dict.Curated())
	want := "I Spoke at WordCamp U.S. in 2025"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestFixesYourCorrectly(t *testing.T) {
	got := MakeTitleCaseStr("it is not your friend", PlainEnglish{}, dict.Curated())
	want := "It Is Not Your Friend"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesOldManAndTheSea(t *testing.T) {
	got := MakeTitleCaseStr("the old man and the sea", PlainEnglish{}, dict.Curated())
	want := "The Old Man and the Sea"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesGreatStoryWithSubtitle(t *testing.T) {
	got := MakeTitleCaseStr("the great story: a tale of two cities", PlainEnglish{}, dict.Curated())
	want := "The Great Story: A Tale of Two Cities"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesLanternAndMoths(t *testing.T) {
	got := MakeTitleCaseStr("lantern flickered; moths began their worship", PlainEnglish{}, dict.Curated())
	want := "Lantern Flickered; Moths Began Their Worship"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesStaticWithGhosts(t *testing.T) {
	got := MakeTitleCaseStr("static filled the room with ghosts", PlainEnglish{}, dict.Curated())
	want := "Static Filled the Room with Ghosts"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesGlassTrembledBeforeThunder(t *testing.T) {
	got := MakeTitleCaseStr("glass trembled before thunder arrived.", PlainEnglish{}, dict.Curated())
	want := "Glass Trembled Before Thunder Arrived."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesHepatitisBShots(t *testing.T) {
	got := MakeTitleCaseStr("an end to hepatitis b shots for all newborns", PlainEnglish{}, dict.Curated())
	want := "An End to Hepatitis B Shots for All Newborns"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesTrumpApprovalRating(t *testing.T) {
	got := MakeTitleCaseStr("trump's approval rating dips as views of his handling of the economy sour", PlainEnglish{}, dict.Curated())
	want := "Trump's Approval Rating Dips as Views of His Handling of the Economy Sour"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesLastDoor(t *testing.T) {
	got := MakeTitleCaseStr("the last door", PlainEnglish{}, dict.Curated())
	want := "The Last Door"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesMidnightRiver(t *testing.T) {
	got := MakeTitleCaseStr("midnight river", PlainEnglish{}, dict.Curated())
	want := "Midnight River"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesAQuietRoom(t *testing.T) {
	got := MakeTitleCaseStr("a quiet room", PlainEnglish{}, dict.Curated())
	want := "A Quiet Room"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesBrokenMap(t *testing.T) {
	got := MakeTitleCaseStr("broken map", PlainEnglish{}, dict.Curated())
	want := "Broken Map"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesFireInAutumn(t *testing.T) {
	got := MakeTitleCaseStr("fire in autumn", PlainEnglish{}, dict.Curated())
	want := "Fire in Autumn"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesHiddenPath(t *testing.T) {
	got := MakeTitleCaseStr("the hidden path", PlainEnglish{}, dict.Curated())
	want := "The Hidden Path"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesUnderBlueSkies(t *testing.T) {
	got := MakeTitleCaseStr("under blue skies", PlainEnglish{}, dict.Curated())
	want := "Under Blue Skies"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesLostAndFound(t *testing.T) {
	got := MakeTitleCaseStr("lost and found", PlainEnglish{}, dict.Curated())
	want := "Lost and Found"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesSilentWatcher(t *testing.T) {
	got := MakeTitleCaseStr("the silent watcher", PlainEnglish{}, dict.Curated())
	want := "The Silent Watcher"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestHandlesWinterRoad(t *testing.T) {
	got := MakeTitleCaseStr("winter road", PlainEnglish{}, dict.Curated())
	want := "Winter Road"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestMaintainsSameApostropheType(t *testing.T) {
	got := MakeTitleCaseStr("Alice’s Adventures in Wonderland", PlainEnglish{}, dict.Curated())
	want := "Alice’s Adventures in Wonderland"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestDoesntLowercaseThisInGithubTemplateTitle(t *testing.T) {
	got := MakeTitleCaseStr("# How Has This Been Tested?", PlainEnglish{}, dict.Curated())
	want := "# How Has This Been Tested?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestLeavesLowercaseAmAfterTime(t *testing.T) {
	got := MakeTitleCaseStr("meeting at 9:05am", PlainEnglish{}, dict.Curated())
	want := "Meeting at 9:05am"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestLeavesUppercaseAmAfterTime(t *testing.T) {
	got := MakeTitleCaseStr("meeting at 9:05AM", PlainEnglish{}, dict.Curated())
	want := "Meeting at 9:05AM"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestLeavesLowercasePmAfterTime(t *testing.T) {
	got := MakeTitleCaseStr("dinner at 7pm", PlainEnglish{}, dict.Curated())
	want := "Dinner at 7pm"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestLeavesUppercasePmAfterTime(t *testing.T) {
	got := MakeTitleCaseStr("dinner at 7PM", PlainEnglish{}, dict.Curated())
	want := "Dinner at 7PM"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestCapitalizesAmWhenNotAfterNumber(t *testing.T) {
	got := MakeTitleCaseStr("i am here", PlainEnglish{}, dict.Curated())
	want := "I Am Here"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestStandaloneXStaysLowercaseBetweenWords(t *testing.T) {
	got := MakeTitleCaseStr("Project 1 x Project 2", PlainEnglish{}, dict.Curated())
	want := "Project 1 x Project 2"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestStandaloneXStaysLowercaseAtStartIsCapitalized(t *testing.T) {
	got := MakeTitleCaseStr("x marks the spot", PlainEnglish{}, dict.Curated())
	want := "X Marks the Spot"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestStandaloneXStaysLowercaseAsLastWordIsCapitalized(t *testing.T) {
	got := MakeTitleCaseStr("project x", PlainEnglish{}, dict.Curated())
	want := "Project X"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
