package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOpenTheLight_FixOpenTheTv(t *testing.T) {
	linttest.AssertSuggestionResult(t, "open the TV", NewOpenTheLight(), "turn on the TV")
}

func TestOpenTheLight_FixHeOpensTheTv(t *testing.T) {
	linttest.AssertSuggestionResult(t, "he opens the TV", NewOpenTheLight(), "he turns on the TV")
}

func TestOpenTheLight_FixSheOpenedTheTv(t *testing.T) {
	linttest.AssertSuggestionResult(t, "she opened the TV", NewOpenTheLight(), "she turned on the TV")
}

func TestOpenTheLight_OpeningTheTv(t *testing.T) {
	linttest.AssertSuggestionResult(t, "opening the TV", NewOpenTheLight(), "turning on the TV")
}

func TestOpenTheLight_DontFlagOpenTheTvApp(t *testing.T) {
	linttest.AssertLintCount(t, "open the TV app", NewOpenTheLight(), 0)
}

func TestOpenTheLight_FixOpenTheTvToWatchTheNews(t *testing.T) {
	linttest.AssertSuggestionResult(t, "open the TV to watch the news", NewOpenTheLight(), "turn on the TV to watch the news")
}

func TestOpenTheLight_FixDontForgetToOpenTheLights(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Don't forget to open the lights when you enter the room.", NewOpenTheLight(), "Don't forget to turn on the lights when you enter the room.")
}

func TestOpenTheLight_FixCanYouOpenTheFan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can you open the fan? It's quite stuffy.", NewOpenTheLight(), "Can you turn on the fan? It's quite stuffy.")
}

func TestOpenTheLight_FixOpenedTheRadio(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I opened the radio to listen to the morning show.", NewOpenTheLight(), "I turned on the radio to listen to the morning show.")
}

func TestOpenTheLight_FixOpenTheAircon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can you open the aircon? It's hot.", NewOpenTheLight(), "Can you turn on the aircon? It's hot.")
}

func TestOpenTheLight_DontFlagOpenTheTvMode(t *testing.T) {
	linttest.AssertLintCount(t, "open the TV mode", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagRadioConfiguration(t *testing.T) {
	linttest.AssertLintCount(t, "To open the Radio Configuration click on the three dots on the top right side.", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpenTheLamp(t *testing.T) {
	t.Skip("Requires much more complex context parsing")
	linttest.AssertLintCount(t, "Now you will need to open your lamp and solder everything together according to schematics.", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpenTvUpTo(t *testing.T) {
	linttest.AssertLintCount(t, "it opens the TV up to a massive library of software", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpenTheLightSlashSound(t *testing.T) {
	t.Skip("Requires more complex context parsing")
	linttest.AssertLintCount(t, "To do so, open the light/sound configuration.", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagCuttingOpen(t *testing.T) {
	t.Skip("Not common enough")
	linttest.AssertLintCount(t, "However, instead of cutting open the lights, I opted to 3D print the Minecraft Torch Nightlight", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpenTheLightSource(t *testing.T) {
	linttest.AssertLintCount(t, "open the light source light and regulate it to the suitable luminance", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpeningLamp(t *testing.T) {
	t.Skip("Requires more complex context parsing")
	linttest.AssertLintCount(t, "After opening the lamp, you need to solder 4 wires to the board in order to connect the USB-to-Serial adapter.", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagFanControl(t *testing.T) {
	linttest.AssertLintCount(t, "It seems like it opens the fan control? ", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpenTvToAccessEeprom(t *testing.T) {
	t.Skip("Requires more complex context parsing")
	linttest.AssertLintCount(t, "Involves opening your TV and directly accessing the an EEPROM IC.", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpenTvViewingApplication(t *testing.T) {
	linttest.AssertLintCount(t, "Open your TV viewing application or platform.", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpenAsNoun(t *testing.T) {
	t.Skip("Requires more complex context parsing")
	linttest.AssertLintCount(t, "when we press open the lamp will be on", NewOpenTheLight(), 0)
}

func TestOpenTheLight_DontFlagOpeningAsNoun(t *testing.T) {
	t.Skip("Requires more complex context parsing")
	linttest.AssertLintCount(t, "and through that opening the light was streaming in", NewOpenTheLight(), 0)
}

func TestOpenTheLight_FixOpeningFan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If the CO2 passed a set point, it would open the fan, and close it once CO2 dropped enough.", NewOpenTheLight(), "If the CO2 passed a set point, it would turn on the fan, and close it once CO2 dropped enough.")
}

func TestOpenTheLight_FixOpeningTv(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This was to prevent me from falling back into the temptation of opening the TV and breaking up the rule I wanted to implement.", NewOpenTheLight(), "This was to prevent me from falling back into the temptation of turning on the TV and breaking up the rule I wanted to implement.")
}

func TestOpenTheLight_DontFlagOpensFanLike(t *testing.T) {
	t.Skip("We don't yet handle hyphenated words")
	linttest.AssertLintCount(t, "Out by the garden fence the high ice plant opens its fan-like petals to the sun.", NewOpenTheLight(), 0)
}

func TestOpenTheLight_FixOpeningLights(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Steering wheel remains blocked until I open my lights.", NewOpenTheLight(), "Steering wheel remains blocked until I turn on my lights.")
}
