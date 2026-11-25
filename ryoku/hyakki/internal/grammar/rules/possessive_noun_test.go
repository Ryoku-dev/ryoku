package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPossessiveNoun_FixesHn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Jennifer went to have seen the ducks cousin.", NewPossessiveNoun(dict.Curated()), "Me and Jennifer went to have seen the duck's cousin.")
}

func TestPossessiveNoun_FixesCatsTail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The cats tail is long.", NewPossessiveNoun(dict.Curated()), "The cat's tail is long.")
}

func TestPossessiveNoun_FixesChildrenToys(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The children toys were scattered.", NewPossessiveNoun(dict.Curated()), "The children's toys were scattered.")
}

func TestPossessiveNoun_FixesTeachersLounge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Many schools have a teachers lounge.", NewPossessiveNoun(dict.Curated()), "Many schools have a teacher's lounge.")
}

func TestPossessiveNoun_FixesDucksPark(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Kids played in the ducks park.", NewPossessiveNoun(dict.Curated()), "Kids played in the duck's park.")
}

func TestPossessiveNoun_NoLintForAlreadyPossessive(t *testing.T) {
	linttest.AssertLintCount(t, "The duck's cousin visited.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_NoLintForAlrreaadyPossessive(t *testing.T) {
	linttest.AssertLintCount(t, "The duck's cousin visited.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_FixesDogsBone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The dogs bone is delicious.", NewPossessiveNoun(dict.Curated()), "The dog's bone is delicious.")
}

func TestPossessiveNoun_FixesStudentsBooks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The students books are on the desk.", NewPossessiveNoun(dict.Curated()), "The student's books are on the desk.")
}

func TestPossessiveNoun_FixesFarmersField(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The farmers field looked beautiful.", NewPossessiveNoun(dict.Curated()), "The farmer's field looked beautiful.")
}

func TestPossessiveNoun_FixesWomenDress(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The women dress was elegant.", NewPossessiveNoun(dict.Curated()), "The women's dress was elegant.")
}

func TestPossessiveNoun_FixesBirdsSong(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We heard the birds song.", NewPossessiveNoun(dict.Curated()), "We heard the bird's song.")
}

func TestPossessiveNoun_FixesScientistsResearch(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The scientists research was groundbreaking.", NewPossessiveNoun(dict.Curated()), "The scientist's research was groundbreaking.")
}

func TestPossessiveNoun_FixesArtistsGallery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The artists gallery is open.", NewPossessiveNoun(dict.Curated()), "The artist's gallery is open.")
}

func TestPossessiveNoun_NoLintForPluralNoun(t *testing.T) {
	linttest.AssertLintCount(t, "The ducks are swimming.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_NoLintForProperNoun(t *testing.T) {
	linttest.AssertLintCount(t, "John's car is red.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_FixesTheStudentsAssignment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The students assignment was due yesterday.", NewPossessiveNoun(dict.Curated()), "The student's assignment was due yesterday.")
}

func TestPossessiveNoun_FixesTheBirdsFlight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The birds flight was graceful.", NewPossessiveNoun(dict.Curated()), "The bird's flight was graceful.")
}

func TestPossessiveNoun_AllowsTheCityLights(t *testing.T) {
	linttest.AssertLintCount(t, "The city lights twinkled in the distance.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_FixesTheFarmersCrops(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The farmers crops were bountiful this year.", NewPossessiveNoun(dict.Curated()), "The farmer's crops were bountiful this year.")
}

func TestPossessiveNoun_FixesTheArtistsInspiration(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The artists inspiration came from nature.", NewPossessiveNoun(dict.Curated()), "The artist's inspiration came from nature.")
}

func TestPossessiveNoun_FixesTheScientistsDiscovery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The scientists discovery revolutionized the field.", NewPossessiveNoun(dict.Curated()), "The scientist's discovery revolutionized the field.")
}

func TestPossessiveNoun_FixesTheWritersNovel(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The writers novel was a bestseller.", NewPossessiveNoun(dict.Curated()), "The writer's novel was a bestseller.")
}

func TestPossessiveNoun_FixesTheStudentsPresentation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The students presentation was well-received.", NewPossessiveNoun(dict.Curated()), "The student's presentation was well-received.")
}

func TestPossessiveNoun_FixesTheTeamsVictory(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The teams victory was celebrated by the fans.", NewPossessiveNoun(dict.Curated()), "The team's victory was celebrated by the fans.")
}

func TestPossessiveNoun_FixesTheMuseumsCollection(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The museums collection included many artifacts.", NewPossessiveNoun(dict.Curated()), "The museum's collection included many artifacts.")
}

func TestPossessiveNoun_NoLintForAlreadyPossessive2(t *testing.T) {
	linttest.AssertLintCount(t, "John's car is red.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_NoLintForProperNoun2(t *testing.T) {
	linttest.AssertLintCount(t, "Mary went to the store.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_FixesTheDoctorsOffice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The doctors office is on Main Street.", NewPossessiveNoun(dict.Curated()), "The doctor's office is on Main Street.")
}

func TestPossessiveNoun_FixesTheNeighborsGarden(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The neighbors garden is beautiful.", NewPossessiveNoun(dict.Curated()), "The neighbor's garden is beautiful.")
}

func TestPossessiveNoun_FixesTheArchitectsDesign(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The architects design was innovative.", NewPossessiveNoun(dict.Curated()), "The architect's design was innovative.")
}

func TestPossessiveNoun_FixesTheBakersShop(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The bakers shop is famous for its bread.", NewPossessiveNoun(dict.Curated()), "The baker's shop is famous for its bread.")
}

func TestPossessiveNoun_FixesTheMusicsPerformance(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The musics performance was captivating.", NewPossessiveNoun(dict.Curated()), "The music's performance was captivating.")
}

func TestPossessiveNoun_FixesTheFlowersScent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The flowers scent filled the room.", NewPossessiveNoun(dict.Curated()), "The flower's scent filled the room.")
}

func TestPossessiveNoun_AllowsBirdsHurried(t *testing.T) {
	linttest.AssertLintCount(t, "The birds hurried off.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_Allows1582HarmsReadability(t *testing.T) {
	t.Skip("false positive issue 1582")
	linttest.AssertLintCount(t, "This harms readability and maintainability.", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_Allows1582ImportsCouples(t *testing.T) {
	t.Skip("false positive issue 1582")
	linttest.AssertLintCount(t, "Since using Webpack syntax in the imports couples the code to a module bundler", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_Allows1582GraphicsProgrammer(t *testing.T) {
	t.Skip("false positive issue 1582")
	linttest.AssertLintCount(t, "Are you a graphics programmer or Rust developer?", NewPossessiveNoun(dict.Curated()), 0)
}

func TestPossessiveNoun_Allows1582DataSources(t *testing.T) {
	t.Skip("false positive issue 1582")
	linttest.AssertLintCount(t, "these data sources can be queried using a full SQL dialect", NewPossessiveNoun(dict.Curated()), 0)
}
