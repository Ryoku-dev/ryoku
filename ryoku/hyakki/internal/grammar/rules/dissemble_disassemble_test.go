package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDissembleDisassemble_FixDissembleSlashAssemble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think it should be implemented in a way that keeps the games in the gme file because there's no real need to dissemble/assemble the whole file", NewDissembleDisassemble(), "I think it should be implemented in a way that keeps the games in the gme file because there's no real need to disassemble/assemble the whole file")
}

func TestDissembleDisassemble_FixDissembleSlashAssemble2(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I think it should be implemented in a way that keeps the games in the gme file because there's no real need to dissemble/assemble the whole file", NewDissembleDisassemble(), []string{
		"I think it should be implemented in a way that keeps the games in the gme file because there's no real need to disassemble/assemble the whole file",
	}, []string{})
}

func TestDissembleDisassemble_FixDecompileReverseEngineerDissemble(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "decompile, reverse-engineer, dissemble, or attempt to derive any source code from the Software", NewDissembleDisassemble(), []string{
		"decompile, reverse-engineer, disassemble, or attempt to derive any source code from the Software",
	}, []string{})
}

func TestDissembleDisassemble_FixDissembledThenAssembled(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "In MosaicKD, these local patterns are first dissembled from OOD data and then assembled to synthesize in-domain data, making OOD-KD feasible.", NewDissembleDisassemble(), []string{
		"In MosaicKD, these local patterns are first disassembled from OOD data and then assembled to synthesize in-domain data, making OOD-KD feasible.",
	}, []string{})
}

func TestDissembleDisassemble_FixDissembledRadio(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Disregard this issue, I dissembled the radio only to find out that the ribbon cable going to the trim switches and other controls was dislodged.", NewDissembleDisassemble(), []string{
		"Disregard this issue, I disassembled the radio only to find out that the ribbon cable going to the trim switches and other controls was dislodged.",
	}, []string{})
}

func TestDissembleDisassemble_FixDissemblingContraption(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I was dissembling a minecart contraption, when my game crashed.", NewDissembleDisassemble(), []string{"I was disassembling a minecart contraption, when my game crashed."}, []string{})
}

func TestDissembleDisassemble_FixDissembleSataUsbDriveEnclosure(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I have try to dissemble the drive and bought a generic SATA to USB enclosure.", NewDissembleDisassemble(), []string{"I have try to disassemble the drive and bought a generic SATA to USB enclosure."}, []string{})
}

func TestDissembleDisassemble_FixDissembleHex(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Hex view can be kept in another tab at the bottom, also dissemble tab might be helpful here.", NewDissembleDisassemble(), []string{
		"Hex view can be kept in another tab at the bottom, also disassemble tab might be helpful here.",
	}, []string{})
}

func TestDissembleDisassemble_FixTitleCaseDissembleAllCapsAsm(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I am looking at the Dissembly window in Visual Studio 2012 and I have the setting for interlacing C++ and generated ASM turned on.", NewDissembleDisassemble(), []string{
		"I am looking at the Disassembly window in Visual Studio 2012 and I have the setting for interlacing C++ and generated ASM turned on.",
	}, []string{})
}

func TestDissembleDisassemble_DontFlagNoTechWords(t *testing.T) {
	linttest.AssertNoLints(t, "Do not dissemble, we each have our own identity. When we say \"ambassador\", we do not mean a populist but a decent person, with a knack for communication", NewDissembleDisassemble())
}

func TestDissembleDisassemble_DontFlagNoApparentTechWords(t *testing.T) {
	linttest.AssertNoLints(t, "Dissembling parachute on Kerbin yields 2k material kits & on Mun it yields 10k.", NewDissembleDisassemble())
}
