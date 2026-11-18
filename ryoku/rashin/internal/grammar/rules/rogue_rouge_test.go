package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRogueRouge_FixGoRouge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Claude can wake up on an idle session easily, go rouge, and blow your cloud credits.", NewRogueRouge(), "Claude can wake up on an idle session easily, go rogue, and blow your cloud credits.")
}

func TestRogueRouge_FixGoesRouge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A \"not considered mature\" incubation stalls out/stops progressing or goes rouge (no longer working with their upstream advisor)?", NewRogueRouge(), "A \"not considered mature\" incubation stalls out/stops progressing or goes rogue (no longer working with their upstream advisor)?")
}

func TestRogueRouge_FixGoingRouge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "port power negotiation and other devices going rouge on the network throwing me off the trail", NewRogueRouge(), "port power negotiation and other devices going rogue on the network throwing me off the trail")
}

func TestRogueRouge_FixGoneRouge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and now the its long term partner has gone rouge", NewRogueRouge(), "and now the its long term partner has gone rogue")
}

func TestRogueRouge_FixBatonRogue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The programs know you dont care about the bronx or baton rogue, you just want a spot", NewRogueRouge(), "The programs know you dont care about the bronx or baton rouge, you just want a spot")
}

func TestRogueRouge_FixKhmerRogue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "After the Khmer Rogue took power they became very aggressive towards Vietnam", NewRogueRouge(), "After the Khmer Rouge took power they became very aggressive towards Vietnam")
}

func TestRogueRouge_FixRougeWho(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What backstory could I use for a rouge who isn't a orphan?", NewRogueRouge(), "What backstory could I use for a rogue who isn't a orphan?")
}

func TestRogueRouge_FixRougeSquadron(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hoth from Star Wars Rouge Squadron.", NewRogueRouge(), "Hoth from Star Wars Rogue Squadron.")
}

func TestRogueRouge_FixRougeNation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In \"Mission Impossible 5: Rouge Nation\" (2015) Benji is seen playing Halo 5.", NewRogueRouge(), "In \"Mission Impossible 5: Rogue Nation\" (2015) Benji is seen playing Halo 5.")
}

func TestRogueRouge_FixRougeNations(t *testing.T) {
	linttest.AssertSuggestionResult(t, "what prevents rouge nations from crediting their banking system with fake USD?", NewRogueRouge(), "what prevents rogue nations from crediting their banking system with fake USD?")
}

func TestRogueRouge_FixLipRogue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The lip rogue and nail henna as well complement the look.", NewRogueRouge(), "The lip rouge and nail henna as well complement the look.")
}

func TestRogueRouge_FixRougeTrader(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm one upgrade away from unlocking the rouge trader is it worth doing and is it beneficial at a low level.", NewRogueRouge(), "I'm one upgrade away from unlocking the rogue trader is it worth doing and is it beneficial at a low level.")
}

func TestRogueRouge_FixRogueLipstick(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Love Dior's Rogue Lipstick-- Hate the Case!", NewRogueRouge(), "Love Dior's Rouge Lipstick-- Hate the Case!")
}

func TestRogueRouge_FixRougeLike(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do you guys think any Rouge-like mechanics would be cool???", NewRogueRouge(), "Do you guys think any Rogue-like mechanics would be cool???")
}

func TestRogueRouge_FixRougeLikes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As someone who dissent really play rouge likes, do you think I'll like risk of rain 2?", NewRogueRouge(), "As someone who dissent really play rogue likes, do you think I'll like risk of rain 2?")
}

func TestRogueRouge_FixRougeTraders(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How do Pirates or Rouge Traders Power Their Gellar Fields?", NewRogueRouge(), "How do Pirates or Rogue Traders Power Their Gellar Fields?")
}
