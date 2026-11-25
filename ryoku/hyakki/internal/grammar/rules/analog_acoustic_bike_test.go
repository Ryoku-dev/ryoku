package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAnalogAcousticBike_FixAcousticBikeUs(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "you might get something that helps you move around your city faster and with less sweat than an acoustic bike", NewAnalogAcousticBike(dict.American), []string{
		"you might get something that helps you move around your city faster and with less sweat than an pedal bike",
	}, []string{
		"you might get something that helps you move around your city faster and with less sweat than an pushbike",
	})
}

func TestAnalogAcousticBike_FixAcousticBikesUk(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Subjectively, it feels as adequately braked as any of my acoustic bikes", NewAnalogAcousticBike(dict.British), []string{
		"Subjectively, it feels as adequately braked as any of my pedal bikes",
		"Subjectively, it feels as adequately braked as any of my pushbikes",
	}, []string{})
}

func TestAnalogAcousticBike_FixAnalogBikeCa(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Pretty sure almost anyone can go faster downhill on an analog bike than the 25kph that Ebike pedal assistance is limited to.", NewAnalogAcousticBike(dict.Canadian), []string{
		"Pretty sure almost anyone can go faster downhill on an pedal bike than the 25kph that Ebike pedal assistance is limited to.",
	}, []string{
		"Pretty sure almost anyone can go faster downhill on an pushbike than the 25kph that Ebike pedal assistance is limited to.",
	})
}

func TestAnalogAcousticBike_FixAnalogBikesAu(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "These bikes are not modified analog bikes — they are ground-up designs built for assisted momentum.", NewAnalogAcousticBike(dict.Australian), []string{
		"These bikes are not modified pedal bikes — they are ground-up designs built for assisted momentum.",
		"These bikes are not modified pushbikes — they are ground-up designs built for assisted momentum.",
	}, []string{})
}

func TestAnalogAcousticBike_FixAcousticBicycleUk(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I still play alcohol powered instruments (double bass, cello), and ride an acoustic bicycle", NewAnalogAcousticBike(dict.British), []string{
		"I still play alcohol powered instruments (double bass, cello), and ride an pedal bicycle",
	}, []string{
		"I still play alcohol powered instruments (double bass, cello), and ride an pushbike",
	})
}

func TestAnalogAcousticBike_FixAnalogBicyclesUs(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "both light electric bicycles and heavy analog bicycles exist", NewAnalogAcousticBike(dict.American), []string{"both light electric bicycles and heavy pedal bicycles exist"}, []string{"both light electric bicycles and heavy pushbikes exist"})
}

func TestAnalogAcousticBike_FixAnalogueBicycleAu(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "There's crossover with a good ol' analogue bicycle, but I found it awkward to rock up at places and events sweating.", NewAnalogAcousticBike(dict.Australian), []string{
		"There's crossover with a good ol' pedal bicycle, but I found it awkward to rock up at places and events sweating.",
		"There's crossover with a good ol' push bicycle, but I found it awkward to rock up at places and events sweating.",
	}, []string{})
}
