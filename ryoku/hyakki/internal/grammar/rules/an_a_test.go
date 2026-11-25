package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAnA_DetectsHtmlAsVowel(t *testing.T) {
	linttest.AssertLintCount(t, "Here is a HTML document.", NewAnA(dict.American), 1)
}

func TestAnA_DetectsLlmAsVowel(t *testing.T) {
	linttest.AssertLintCount(t, "Here is a LLM document.", NewAnA(dict.American), 1)
}

func TestAnA_DetectsLlmHyphenAsVowel(t *testing.T) {
	linttest.AssertLintCount(t, "Here is a LLM-based system.", NewAnA(dict.American), 1)
}

func TestAnA_DetectsEulerAsVowel(t *testing.T) {
	linttest.AssertLintCount(t, "This is an Euler brick.", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "The graph has an Eulerian tour.", NewAnA(dict.American), 0)
}

func TestAnA_CapitalizedFourier(t *testing.T) {
	linttest.AssertLintCount(t, "Then, perform a Fourier transform.", NewAnA(dict.American), 0)
}

func TestAnA_OnceOver(t *testing.T) {
	linttest.AssertLintCount(t, "give this a once-over.", NewAnA(dict.American), 0)
}

func TestAnA_Issue196(t *testing.T) {
	linttest.AssertLintCount(t, "This is formatted as an `ext4` file system.", NewAnA(dict.American), 0)
}

func TestAnA_AllowsLowercaseVowels(t *testing.T) {
	linttest.AssertLintCount(t, "not an error", NewAnA(dict.American), 0)
}

func TestAnA_AllowsLowercaseConsonants(t *testing.T) {
	linttest.AssertLintCount(t, "not a crash", NewAnA(dict.American), 0)
}

func TestAnA_DisallowsLowercaseVowels(t *testing.T) {
	linttest.AssertLintCount(t, "not a error", NewAnA(dict.American), 1)
}

func TestAnA_DisallowsLowercaseConsonants(t *testing.T) {
	linttest.AssertLintCount(t, "not an crash", NewAnA(dict.American), 1)
}

func TestAnA_AllowsUppercaseVowels(t *testing.T) {
	linttest.AssertLintCount(t, "not an Error", NewAnA(dict.American), 0)
}

func TestAnA_AllowsUppercaseConsonants(t *testing.T) {
	linttest.AssertLintCount(t, "not a Crash", NewAnA(dict.American), 0)
}

func TestAnA_DisallowsUppercaseVowels(t *testing.T) {
	linttest.AssertLintCount(t, "not a Error", NewAnA(dict.American), 1)
}

func TestAnA_DisallowsUppercaseConsonants(t *testing.T) {
	linttest.AssertLintCount(t, "not an Crash", NewAnA(dict.American), 1)
}

func TestAnA_DisallowsAInterface(t *testing.T) {
	linttest.AssertLintCount(t, "A interface for an object that can perform linting actions.", NewAnA(dict.American), 1)
}

func TestAnA_AllowIssue751(t *testing.T) {
	linttest.AssertLintCount(t, "He got a 52% approval rating.", NewAnA(dict.American), 0)
}

func TestAnA_AllowAnMpAndAnMp3(t *testing.T) {
	linttest.AssertLintCount(t, "an MP and an MP3?", NewAnA(dict.American), 0)
}

func TestAnA_AllowAnLowercaseMp3(t *testing.T) {
	linttest.AssertLintCount(t, "an mp3 file", NewAnA(dict.American), 0)
}

func TestAnA_DisallowAMpAndAMp3(t *testing.T) {
	linttest.AssertLintCount(t, "a MP and a MP3?", NewAnA(dict.American), 2)
}

func TestAnA_DisallowALowercaseMp3(t *testing.T) {
	linttest.AssertLintCount(t, "a mp3 file", NewAnA(dict.American), 1)
}

func TestAnA_RecognizeAcronyms(t *testing.T) {
	linttest.AssertLintCount(t, "using a MAC address", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a NASA spacecraft", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a NAT", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a REST API", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a LIBERO", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a README", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a LAN", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an RA message", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an SI unit", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "he is an MA of both Oxford and Cambridge", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "in an FA Cup 6th Round match", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a AM transmitter", NewAnA(dict.American), 1)
}

func TestAnA_ReadsEmphasisCapsAsWords(t *testing.T) {
	linttest.AssertLintCount(t, "a FREE event", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "James Bond deals with a SPECTRE agent", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a FLASH based filing system", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a SYN flood attack", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a STUN server", NewAnA(dict.American), 0)
}

func TestAnA_KeepsInitialismsAndRomanNumerals(t *testing.T) {
	linttest.AssertLintCount(t, "an SSH tunnel", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an FTP server", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an FTPS connection", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an FPS multiplayer game", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an MPG figure", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an ML research project", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an XVI century painting", NewAnA(dict.American), 0)
}

func TestAnA_DontMisrecognizeAsAcronym(t *testing.T) {
	linttest.AssertLintCount(t, "a UPD connection", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a UPB device", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a UPS or power device", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a USB 2.0 port", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "an HEVC HLS stream", NewAnA(dict.American), 0)
}

func TestAnA_AUdev(t *testing.T) {
	linttest.AssertLintCount(t, "a udev rule", NewAnA(dict.American), 0)
}

func TestAnA_AnMdns(t *testing.T) {
	linttest.AssertLintCount(t, "an mDNS tool", NewAnA(dict.American), 0)
}

func TestAnA_AnRflink(t *testing.T) {
	linttest.AssertLintCount(t, "an RFLink device", NewAnA(dict.American), 0)
}

func TestAnA_AnFfmpeg(t *testing.T) {
	linttest.AssertLintCount(t, "an FFmpeg-compatible input file", NewAnA(dict.American), 0)
}

func TestAnA_AHoney(t *testing.T) {
	linttest.AssertLintCount(t, "a Honeywell alarm panel", NewAnA(dict.American), 0)
}

func TestAnA_AnOnedrive(t *testing.T) {
	linttest.AssertLintCount(t, "a OneDrive folder", NewAnA(dict.American), 0)
}

func TestAnA_AUbiquiti(t *testing.T) {
	linttest.AssertLintCount(t, "a Ubiquiti UniFi Network application", NewAnA(dict.American), 0)
}

func TestAnA_AnHonest(t *testing.T) {
	linttest.AssertLintCount(t, "an honest mistake", NewAnA(dict.American), 0)
}

func TestAnA_DontFlagAnHerbForAmerican(t *testing.T) {
	linttest.AssertLintCount(t, "an herb", NewAnA(dict.American), 0)
}

func TestAnA_DontFlagAHerbForBritish(t *testing.T) {
	linttest.AssertLintCount(t, "a herb", NewAnA(dict.British), 0)
}

func TestAnA_CorrectAnHerbForAustralian(t *testing.T) {
	linttest.AssertSuggestionResult(t, "an herb", NewAnA(dict.Australian), "a herb")
}

func TestAnA_CorrectAHerbForCanadian(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a herb", NewAnA(dict.Canadian), "an herb")
}

func TestAnA_DontFlagASql(t *testing.T) {
	linttest.AssertLintCount(t, "a SQL query", NewAnA(dict.American), 0)
}

func TestAnA_DontFlagAnSql(t *testing.T) {
	linttest.AssertLintCount(t, "an SQL query", NewAnA(dict.Australian), 0)
}

func TestAnA_AllowAnAndAForLed2550(t *testing.T) {
	linttest.AssertLintCount(t, "an LED", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "a LED", NewAnA(dict.American), 0)
}

func TestAnA_AllowAAndAnForUrl(t *testing.T) {
	linttest.AssertLintCount(t, "I pronounce URL as 'yoo-are-ell' so for me it's 'a URL'", NewAnA(dict.American), 0)
	linttest.AssertLintCount(t, "But some people pronounce it like 'earl' so for them it's 'an URL'", NewAnA(dict.American), 0)
}

func TestAnA_DontFlagAFGradingScale3715(t *testing.T) {
	linttest.AssertNoLints(t, "an A-F grading scale", NewAnA(dict.American))
}

func TestAnA_DontFlagIs80AnA3715(t *testing.T) {
	linttest.AssertNoLints(t, "Is 80 an A in Malaysia?", NewAnA(dict.American))
}

func TestAnA_DontFlagAnNpm4139(t *testing.T) {
	linttest.AssertNoLints(t, "Why lib/ or light-base/ alone forces an npm bump", NewAnA(dict.American))
}

func TestAnA_CorrectANpm4139(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why lib/ or light-base/ alone forces a npm bump", NewAnA(dict.American), "Why lib/ or light-base/ alone forces an npm bump")
}
