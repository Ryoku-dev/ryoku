package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRedundantAcronyms_TestMadeUp(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I forgot my PIN number!", NewRedundantAcronyms(), []string{
		"I forgot my PIN!",
		"I forgot my personal identification number!",
	}, []string{})
}

func TestRedundantAcronyms_TestAllCapsSingular(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "CAN TWO CARS HAVE THE SAME VIN NUMBER?", NewRedundantAcronyms(), []string{
		"CAN TWO CARS HAVE THE SAME VIN?",
		"CAN TWO CARS HAVE THE SAME VEHICLE IDENTIFICATION NUMBER?",
	}, []string{})
}

func TestRedundantAcronyms_TestAllCapsPlural(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "THESE ATM MACHINES ALL HAVE HIGH FEES!", NewRedundantAcronyms(), []string{
		"THESE ATMS ALL HAVE HIGH FEES!",
		"THESE AUTOMATED TELLER MACHINES ALL HAVE HIGH FEES!",
	}, []string{})
}

func TestRedundantAcronyms_TestAllLowercaseSingular(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "the atm machine at my card", NewRedundantAcronyms(), []string{
		"the atm at my card",
		"the automated teller machine at my card",
	}, []string{})
}

func TestRedundantAcronyms_TestAllLowercasePlural(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "gui interfaces were sooo trendy in 1984!", NewRedundantAcronyms(), []string{
		"guis were sooo trendy in 1984!",
		"graphical user interfaces were sooo trendy in 1984!",
	}, []string{})
}

func TestRedundantAcronyms_CorrectAtmMachine(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Developed an ATM machine application for Raspberry Pi", NewRedundantAcronyms(), []string{
		"Developed an ATM application for Raspberry Pi",
		"Developed an automatic teller machine application for Raspberry Pi",
		"Developed an automated teller machine application for Raspberry Pi",
	}, []string{})
}

func TestRedundantAcronyms_CorrectAtmMachines(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "ATM machines allow 4 or 6 digit PIN codes", NewRedundantAcronyms(), []string{
		"ATMs allow 4 or 6 digit PIN codes",
		"automated teller machines allow 4 or 6 digit PIN codes",
		"automatic teller machines allow 4 or 6 digit PIN codes",
	}, []string{})
}

func TestRedundantAcronyms_CorrectGuiInterface(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "This project develops using java language with GUI interface.", NewRedundantAcronyms(), []string{
		"This project develops using java language with GUI.",
		"This project develops using java language with graphical user interface.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectGuiInterfaces(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "In non-crafting GUI interfaces, such as a mod's own recipe tree, the shortcut key cannot be used to view item usage or crafting methods.", NewRedundantAcronyms(), []string{
		"In non-crafting GUIs, such as a mod's own recipe tree, the shortcut key cannot be used to view item usage or crafting methods.",
		"In non-crafting graphical user interfaces, such as a mod's own recipe tree, the shortcut key cannot be used to view item usage or crafting methods.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectLcdDisplay(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "This function accepts I2C shield address for LCD display, number of columns, rows and dot size", NewRedundantAcronyms(), []string{
		"This function accepts I2C shield address for LCD, number of columns, rows and dot size",
		"This function accepts I2C shield address for liquid crystal display, number of columns, rows and dot size",
	}, []string{})
}

func TestRedundantAcronyms_CorrectLcdDisplays(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "ScreenUi makes it easy to build simple or complex character based user interfaces on small LCD displays like those commonly used with Arduinos.", NewRedundantAcronyms(), []string{
		"ScreenUi makes it easy to build simple or complex character based user interfaces on small LCDs like those commonly used with Arduinos.",
		"ScreenUi makes it easy to build simple or complex character based user interfaces on small liquid crystal displays like those commonly used with Arduinos.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectPinNumbersCaps(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Randomly generating PIN numbers for ATM access.", NewRedundantAcronyms(), []string{
		"Randomly generating PINs for ATM access.",
		"Randomly generating personal identification numbers for ATM access.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectPinNumberAllCaps(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "DON'T LET ANYONE SEE YOUR PIN NUMBER", NewRedundantAcronyms(), []string{
		"DON'T LET ANYONE SEE YOUR PIN",
		"DON'T LET ANYONE SEE YOUR PERSONAL IDENTIFICATION NUMBER",
	}, []string{})
}

func TestRedundantAcronyms_DontCorrectPinNumberLowercase(t *testing.T) {
	linttest.AssertNoLints(t, "GPIO 26 (pin 37) on the Pi4 is mapped to pin nummer GPIO 425 on the pi5", NewRedundantAcronyms())
}

func TestRedundantAcronyms_DontCorrectPinNumberTitlecase(t *testing.T) {
	linttest.AssertNoLints(t, "Pin Number Match Project in Javascript.", NewRedundantAcronyms())
}

func TestRedundantAcronyms_CorrectTuiInterface(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Could a history search TUI interface be added for xonsh?", NewRedundantAcronyms(), []string{
		"Could a history search TUI be added for xonsh?",
		"Could a history search text-based user interface be added for xonsh?",
		"Could a history search terminal user interface be added for xonsh?",
	}, []string{})
}

func TestRedundantAcronyms_CorrectUiInterface(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "call ESPUI.begin(\"Some Title\"); to start the UI interface", NewRedundantAcronyms(), []string{
		"call ESPUI.begin(\"Some Title\"); to start the UI",
		"call ESPUI.begin(\"Some Title\"); to start the user interface",
	}, []string{})
}

func TestRedundantAcronyms_CorrectVinNumbers(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "That was actually accurate in decoding the VIN numbers but it costed me a 1000 USD.", NewRedundantAcronyms(), []string{
		"That was actually accurate in decoding the VINs but it costed me a 1000 USD.",
		"That was actually accurate in decoding the vehicle identification numbers but it costed me a 1000 USD.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectVinNumber(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "we have implemented verification algorithms, which ensure that VIN number received is correct", NewRedundantAcronyms(), []string{
		"we have implemented verification algorithms, which ensure that VIN received is correct",
		"we have implemented verification algorithms, which ensure that vehicle identification number received is correct",
	}, []string{})
}

func TestRedundantAcronyms_CorrectGopParty(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "If it wasnt for bribery and blackmail there wouldnt be  a GOP party", NewRedundantAcronyms(), []string{
		"If it wasnt for bribery and blackmail there wouldnt be  a Grand Old Party",
		"If it wasnt for bribery and blackmail there wouldnt be  a GOP",
	}, []string{})
}

func TestRedundantAcronyms_CorrectLlmModel(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "They end up attributing a lot of what they're doing to the capabilities of the LLM model.", NewRedundantAcronyms(), []string{
		"They end up attributing a lot of what they're doing to the capabilities of the LLM.",
		"They end up attributing a lot of what they're doing to the capabilities of the large language model.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectLlmModels(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Ollama is an open-source tool for running LLM models locally.", NewRedundantAcronyms(), []string{
		"Ollama is an open-source tool for running LLMs locally.",
		"Ollama is an open-source tool for running large language models locally.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectUsd(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "And because I didn't have USD dollars, I used zip ties [music] instead of a proper microonal bass.", NewRedundantAcronyms(), []string{
		"And because I didn't have US dollars, I used zip ties [music] instead of a proper microonal bass.",
		"And because I didn't have USD, I used zip ties [music] instead of a proper microonal bass.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectPcbBoard(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "The PCB board is based on an STM32F microcontroller.", NewRedundantAcronyms(), []string{
		"The PCB is based on an STM32F microcontroller.",
		"The printed circuit board is based on an STM32F microcontroller.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectPcbBoards(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "course project aimed to classify PCB boards as defect or non-defect - ChethanaVaisali/PCB_Classification.", NewRedundantAcronyms(), []string{
		"course project aimed to classify PCBs as defect or non-defect - ChethanaVaisali/PCB_Classification.",
		"course project aimed to classify printed circuit boards as defect or non-defect - ChethanaVaisali/PCB_Classification.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectBwtTransform(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "TSuffix Arrays: It turns out the way we generated the BWT transform above was quite inefficient.", NewRedundantAcronyms(), []string{
		"TSuffix Arrays: It turns out the way we generated the BWT above was quite inefficient.",
		"TSuffix Arrays: It turns out the way we generated the Burrows-Wheeler transform above was quite inefficient.",
	}, []string{})
}

func TestRedundantAcronyms_CorrectFftTransform(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "In fact, the FFT transform does not have to process functions of equally spaced time samples", NewRedundantAcronyms(), []string{
		"In fact, the FFT does not have to process functions of equally spaced time samples",
		"In fact, the Fast Fourier transform does not have to process functions of equally spaced time samples",
	}, []string{})
}
