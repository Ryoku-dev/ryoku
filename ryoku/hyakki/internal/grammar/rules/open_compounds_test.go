package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOpenCompounds_CorrectsLoneInfront(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Button always overlaps (infront) of other views.", NewOpenCompounds(), "Button always overlaps (in front) of other views.")
}

func TestOpenCompounds_CorrectsInfront(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So if i have no variable or a running process id/name which indicates that liveley is infront/fullscreen i can't do anything further via batch and must wait ...", NewOpenCompounds(), "So if i have no variable or a running process id/name which indicates that liveley is in front/fullscreen i can't do anything further via batch and must wait ...")
}

func TestOpenCompounds_IgnoresPascalcase(t *testing.T) {
	linttest.AssertLintCount(t, "InFront Labs, LLC has 16 repositories available. Follow their code on GitHub.", NewOpenCompounds(), 0)
}

func TestOpenCompounds_IgnoresCamelcase(t *testing.T) {
	linttest.AssertLintCount(t, "Click the \"toggle\" button to see how wrapping changes when an inFront is added to a letter in a word.", NewOpenCompounds(), 0)
}

func TestOpenCompounds_CorrectWithPeriodAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Car with a reversed ramp infront.", NewOpenCompounds(), "Car with a reversed ramp in front.")
}

func TestOpenCompounds_IgnoreHyphenBefore(t *testing.T) {
	linttest.AssertLintCount(t, "-infront", NewOpenCompounds(), 0)
}

func TestOpenCompounds_IgnoreHyphenAfter(t *testing.T) {
	linttest.AssertLintCount(t, "infront-", NewOpenCompounds(), 0)
}

func TestOpenCompounds_IgnoresWithHyphenBefore(t *testing.T) {
	linttest.AssertLintCount(t, "Instantly share code, notes, and snippets. @yossi-infront", NewOpenCompounds(), 0)
}

func TestOpenCompounds_IgnoresWithHyphenAfter(t *testing.T) {
	linttest.AssertLintCount(t, "infront-cycle.ipe · infront-cycle.ipe · infront-cycle.svg · infront-cycle.svg · infront-s1s2.ipe · infront-s1s2.ipe · infront-s1s2.svg · infront-s1s2.svg.", NewOpenCompounds(), 0)
}

func TestOpenCompounds_EvenRepeatedInfrontWorks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "infront infront", NewOpenCompounds(), "in front in front")
}

func TestOpenCompounds_CorrectAfewAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ITK code to generate anisotropic metrics, mostly Riemannian metrics and afew particular cases of Finslerian metrics.", NewOpenCompounds(), "ITK code to generate anisotropic metrics, mostly Riemannian metrics and a few particular cases of Finslerian metrics.")
}

func TestOpenCompounds_CorrectAlotAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Alot", NewOpenCompounds(), "A lot")
}

func TestOpenCompounds_CorrectAswellAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Aswell", NewOpenCompounds(), "As well")
}

func TestOpenCompounds_CorrectsAsKeyboardsAswell(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tool to read physical joystick devices, keyboards aswell, and create virtual joystick devices and output keyboard presses on a Linux system.", NewOpenCompounds(), "Tool to read physical joystick devices, keyboards as well, and create virtual joystick devices and output keyboard presses on a Linux system.")
}

func TestOpenCompounds_CorrectsAswellAs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When UseAcrylic is true in Focused aswell as Unfocused Apearance , changing enableUnfocusedAcrylic at runtime doesn't work", NewOpenCompounds(), "When UseAcrylic is true in Focused as well as Unfocused Apearance , changing enableUnfocusedAcrylic at runtime doesn't work")
}

func TestOpenCompounds_CorrectsTomlAswell(t *testing.T) {
	linttest.AssertSuggestionResult(t, "format Cargo.toml aswell #5893 - rust-lang/rustfmt", NewOpenCompounds(), "format Cargo.toml as well #5893 - rust-lang/rustfmt")
}

func TestOpenCompounds_CorrectAswell(t *testing.T) {
	linttest.AssertSuggestionResult(t, "'wejoy' is a tool to read physical joystick devices, aswell as keyboards, create virtual joystick devices and output keyboard presses on a Linux system.", NewOpenCompounds(), "'wejoy' is a tool to read physical joystick devices, as well as keyboards, create virtual joystick devices and output keyboard presses on a Linux system.")
}

func TestOpenCompounds_CorrectAtall(t *testing.T) {
	linttest.AssertSuggestionResult(t, "claude code with vs code extension not working atall", NewOpenCompounds(), "claude code with vs code extension not working at all")
}

func TestOpenCompounds_CorrectAtleastAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Atleast", NewOpenCompounds(), "At least")
}

func TestOpenCompounds_IgnoreAtleastPascalcase(t *testing.T) {
	linttest.AssertLintCount(t, "I want to understand if we are using AtLeast correctly.", NewOpenCompounds(), 0)
}

func TestOpenCompounds_IgnoreAtleastCamelcase(t *testing.T) {
	linttest.AssertLintCount(t, "verfiy with atLeast = 0 should pass even if the mocked function is never called.", NewOpenCompounds(), 0)
}

func TestOpenCompounds_CorrectAtleast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Mar 22, 2562 BE — constructor - expected atleast one input #250.", NewOpenCompounds(), "Mar 22, 2562 BE — constructor - expected at least one input #250.")
}

func TestOpenCompounds_CorrectEachotherAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Eachother", NewOpenCompounds(), "Each other")
}

func TestOpenCompounds_CorrectEachother(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Script parsing fails when two scenes reference eachother", NewOpenCompounds(), "Script parsing fails when two scenes reference each other")
}

func TestOpenCompounds_CorrectIncaseAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Incase", NewOpenCompounds(), "In case")
}

func TestOpenCompounds_CorrectInCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Support for enum variable incase of reusable enum class", NewOpenCompounds(), "Support for enum variable in case of reusable enum class")
}

func TestOpenCompounds_IgnoreIncasePascalcase(t *testing.T) {
	linttest.AssertLintCount(t, "InCase save your secrets for a friend, so they can use in case it in case you went \"missing\".", NewOpenCompounds(), 0)
}

func TestOpenCompounds_CorrectInfactAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes I do infact exist :O", NewOpenCompounds(), "Yes I do in fact exist :O")
}

func TestOpenCompounds_CorrectUpto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Free for upto 10k subscribers, unlimited push notifications, in-browser messaging", NewOpenCompounds(), "Free for up to 10k subscribers, unlimited push notifications, in-browser messaging")
}
