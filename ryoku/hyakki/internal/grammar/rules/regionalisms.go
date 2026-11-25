package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

import (
	"fmt"
	"slices"
	"strings"
)

// regionalismsTerm represents a single entry in the regional terms database.
type regionalismsTerm struct {
	term     string
	flag     bool
	dialects []dict.Dialect
	concept  string
}

var regionalismsTerms = []regionalismsTerm{
	{term: "aubergine", flag: true, dialects: []dict.Dialect{dict.British}, concept: "AubergineBrinjalEggplant"},
	{term: "aubergines", flag: true, dialects: []dict.Dialect{dict.British}, concept: "AuberginesBrinjalsEggplants"},
	{term: "Bharat", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "BharatIndia"},
	{term: "brinjal", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "AubergineBrinjalEggplant"},
	{term: "brinjals", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "AuberginesBrinjalsEggplants"},
	{term: "Brit", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian}, concept: "BritBritisher"},
	{term: "Brits", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian}, concept: "BritsBritishers"},
	{term: "Britisher", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "BritBritisher"},
	{term: "Britishers", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "BritsBritishers"},
	{term: "blood nose", flag: true, dialects: []dict.Dialect{dict.Australian}, concept: "BloodNoseNosebleed"},
	{term: "bum bag", flag: true, dialects: []dict.Dialect{dict.Australian}, concept: "BumBagFannyPack"},
	{term: "burglarize", flag: true, dialects: []dict.Dialect{dict.American}, concept: "BurglarizeBurgle"},
	{term: "burgle", flag: true, dialects: []dict.Dialect{dict.British}, concept: "BurglarizeBurgle"},
	{term: "campervan", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "CampervanRv"},
	{term: "caravan", flag: false, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "CaravanTrailer"},
	{term: "catsup", flag: true, dialects: []dict.Dialect{dict.American}, concept: "CatsupKetchupTomatoSauce"},
	{term: "cellphone", flag: true, dialects: []dict.Dialect{dict.American}, concept: "CellPhoneMobilePhone"},
	{term: "check", flag: false, dialects: []dict.Dialect{dict.American}, concept: "CheckCheque"},
	{term: "checkbook", flag: true, dialects: []dict.Dialect{dict.American}, concept: "CheckbookChequebook"},
	{term: "cheque", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British, dict.Canadian, dict.Indian}, concept: "CheckCheque"},
	{term: "chequebook", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British, dict.Canadian, dict.Indian}, concept: "CheckbookChequebook"},
	{term: "chips", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian}, concept: "ChipsCrisps"},
	{term: "cilantro", flag: true, dialects: []dict.Dialect{dict.American}, concept: "CilantroCoriander"},
	{term: "coolbox", flag: true, dialects: []dict.Dialect{dict.British}, concept: "CoolboxCoolerEsky"},
	{term: "cooler", flag: false, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "CoolboxCoolerEsky"},
	{term: "coriander", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "CilantroCoriander"},
	{term: "crisps", flag: true, dialects: []dict.Dialect{dict.British}, concept: "ChipsCrisps"},
	{term: "crore", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "Crore"},
	{term: "crores", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "Crores"},
	{term: "diaper", flag: true, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "DiaperNappy"},
	{term: "doona", flag: true, dialects: []dict.Dialect{dict.Australian}, concept: "DoonaDuvet"},
	{term: "dummy", flag: false, dialects: []dict.Dialect{dict.Australian}, concept: "DummyPacifier"},
	{term: "duvet", flag: true, dialects: []dict.Dialect{dict.Australian}, concept: "DoonaDuvet"},
	{term: "eggplant", flag: true, dialects: []dict.Dialect{dict.American, dict.Australian}, concept: "AubergineBrinjalEggplant"},
	{term: "eggplants", flag: true, dialects: []dict.Dialect{dict.American, dict.Australian}, concept: "AuberginesBrinjalsEggplants"},
	{term: "esky", flag: true, dialects: []dict.Dialect{dict.Australian}, concept: "CoolboxCoolerEsky"},
	{term: "estate", flag: false, dialects: []dict.Dialect{dict.British}, concept: "StationWagonEstate"},
	{term: "fanny pack", flag: true, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "BumBagFannyPack"},
	{term: "faucet", flag: true, dialects: []dict.Dialect{dict.American}, concept: "FaucetTap"},
	{term: "flashlight", flag: true, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "FlashlightTorch"},
	{term: "football", flag: false, dialects: []dict.Dialect{dict.British}, concept: "FootballSoccer"},
	{term: "footpath", flag: true, dialects: []dict.Dialect{dict.Australian}, concept: "FootpathPavementSidewalk"},
	{term: "gas", flag: false, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "GasolinePetrol"},
	{term: "gas station", flag: true, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "GasStationPetrolStationServiceStation"},
	{term: "gasoline", flag: true, dialects: []dict.Dialect{dict.American}, concept: "GasolinePetrol"},
	{term: "godown", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "GodownWarehouse"},
	{term: "India", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian, dict.Indian}, concept: "BharatIndia"},
	{term: "jumper", flag: false, dialects: []dict.Dialect{dict.Australian}, concept: "JumperSweater"},
	{term: "ketchup", flag: true, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "CatsupKetchupTomatoSauce"},
	{term: "lakh", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "Lakh"},
	{term: "lakhs", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "Lakhs"},
	{term: "light bulb", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian}, concept: "LightBulbLightGlobe"},
	{term: "light globe", flag: true, dialects: []dict.Dialect{dict.Australian}, concept: "LightBulbLightGlobe"},
	{term: "lorry", flag: true, dialects: []dict.Dialect{dict.British}, concept: "LorryTruck"},
	{term: "mobile phone", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "CellPhoneMobilePhone"},
	{term: "motorhome", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "MotorhomeRv"},
	{term: "nappy", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "DiaperNappy"},
	{term: "nosebleed", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian}, concept: "BloodNoseNosebleed"},
	{term: "nought", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British, dict.Canadian, dict.Indian}, concept: "NoughtZero"},
	{term: "pacifier", flag: true, dialects: []dict.Dialect{dict.American}, concept: "DummyPacifier"},
	{term: "pavement", flag: false, dialects: []dict.Dialect{dict.British}, concept: "FootpathPavementSidewalk"},
	{term: "petrol", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "GasolinePetrol"},
	{term: "petrol station", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "GasStationPetrolStationServiceStation"},
	{term: "photocopier", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British, dict.Canadian}, concept: "PhotocopierXerox"},
	{term: "photocopy", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British, dict.Canadian}, concept: "PhotocopyXerox"},
	{term: "pickup truck", flag: true, dialects: []dict.Dialect{dict.American}, concept: "PickupUte"},
	{term: "postal code", flag: false, dialects: []dict.Dialect{dict.Canadian, dict.Indian}, concept: "PostalCodePostcodeZipCode"},
	{term: "postcode", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "PostalCodePostcodeZipCode"},
	{term: "pram", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "PramStroller"},
	{term: "prepone", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "Prepone"},
	{term: "rv", flag: true, dialects: []dict.Dialect{dict.American}, concept: "CampervanRv"},
	{term: "rv", flag: true, dialects: []dict.Dialect{dict.American}, concept: "MotorhomeRv"},
	{term: "sidewalk", flag: true, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "FootpathPavementSidewalk"},
	{term: "soccer", flag: true, dialects: []dict.Dialect{dict.American, dict.Australian}, concept: "FootballSoccer"},
	{term: "spanner", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "SpannerWrench"},
	{term: "station wagon", flag: true, dialects: []dict.Dialect{dict.American, dict.Australian}, concept: "StationWagonEstate"},
	{term: "stroller", flag: true, dialects: []dict.Dialect{dict.American, dict.Australian}, concept: "PramStroller"},
	{term: "sweater", flag: true, dialects: []dict.Dialect{dict.American}, concept: "JumperSweater"},
	{term: "tap", flag: false, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "FaucetTap"},
	{term: "tire", flag: false, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "TireTyre"},
	{term: "tyre", flag: true, dialects: []dict.Dialect{dict.Australian, dict.British, dict.Indian}, concept: "TireTyre"},
	{term: "tomato sauce", flag: false, dialects: []dict.Dialect{dict.Australian}, concept: "CatsupKetchupTomatoSauce"},
	{term: "torch", flag: false, dialects: []dict.Dialect{dict.Australian, dict.British}, concept: "FlashlightTorch"},
	{term: "trailer", flag: false, dialects: []dict.Dialect{dict.American}, concept: "CaravanTrailer"},
	{term: "truck", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.Canadian}, concept: "LorryTruck"},
	{term: "update", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian}, concept: "UpdateUpdation"},
	{term: "updates", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian}, concept: "UpdateUpdation"},
	{term: "updation", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "UpdateUpdation"},
	{term: "updations", flag: true, dialects: []dict.Dialect{dict.Indian}, concept: "UpdatesUpdations"},
	{term: "ute", flag: true, dialects: []dict.Dialect{dict.Australian}, concept: "PickupUte"},
	{term: "xerox", flag: true, dialects: []dict.Dialect{dict.American}, concept: "PhotocopierXerox"},
	{term: "xerox", flag: true, dialects: []dict.Dialect{dict.American}, concept: "PhotocopyXerox"},
	{term: "warehouse", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian}, concept: "GodownWarehouse"},
	{term: "wrench", flag: true, dialects: []dict.Dialect{dict.American}, concept: "SpannerWrench"},
	{term: "windscreen", flag: true, dialects: []dict.Dialect{dict.British, dict.Australian}, concept: "WindscreenWindshield"},
	{term: "windshield", flag: true, dialects: []dict.Dialect{dict.American, dict.Canadian}, concept: "WindscreenWindshield"},
	{term: "zero", flag: false, dialects: []dict.Dialect{dict.American, dict.Australian, dict.British, dict.Canadian, dict.Indian}, concept: "NoughtZero"},
	{term: "zip code", flag: true, dialects: []dict.Dialect{dict.American}, concept: "PostalCodePostcodeZipCode"},
}

type Regionalisms struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewRegionalisms(dialect dict.Dialect) *Regionalisms {
	e := expr.NewFirstMatchOf()
	for _, row := range regionalismsTerms {
		if row.flag {
			e.Add(expr.NewFixedPhraseFromPhrase(row.term))
		}
	}
	return &Regionalisms{e, dialect}
}
func (r *Regionalisms) Expr() expr.Expr                  { return r.expression }
func (r *Regionalisms) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Regionalisms) Description() string              { return "Regionalisms" }
func (r *Regionalisms) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	flagged := strings.ToLower(span.GetContentString(src))
	concept := ""
	for _, row := range regionalismsTerms {
		if row.term == flagged {
			if slices.Contains(row.dialects, r.dialect) {
				return nil
			}
			if concept == "" {
				concept = row.concept
			}
		}
	}
	if concept == "" {
		return nil
	}
	var other []string
	var suggestions []lint.Suggestion
	for _, row := range regionalismsTerms {
		if row.concept == concept && slices.Contains(row.dialects, r.dialect) {
			other = append(other, row.term)
			suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(row.term, span.GetContent(src)))
		}
	}
	message := fmt.Sprintf("`%s` isn't used in %s English.", flagged, r.dialect)
	if len(other) == 1 {
		message = fmt.Sprintf("`%s` isn't used in %s English. Use `%s` instead.", flagged, r.dialect, other[0])
	}
	return &lint.Lint{Span: span, LintKind: lint.Regionalism, Suggestions: suggestions, Message: message, Priority: 64}
}
