package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPleadedPled_CorrectPleadedToPled(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Reality Winner pleaded guilty in June to a single count of transmitting national security information.", NewPreferPled(), "Reality Winner pled guilty in June to a single count of transmitting national security information.")
}

func TestPleadedPled_CorrectPledToPleaded(t *testing.T) {
	linttest.AssertSuggestionResult(t, "2005 Samsung pled guilty in connection with the cartel and paid a fine.", NewPreferPleaded(), "2005 Samsung pleaded guilty in connection with the cartel and paid a fine.")
}
