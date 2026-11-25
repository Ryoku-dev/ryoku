package structures

import "ryoku-rashin/internal/protocol"

var defaultCatalog = buildBuiltin()

// Builtin returns fresh immutable-default snapshots, including complete worked
// examples. Callers customize through Store.Clone rather than modifying defaults.
func Builtin() []protocol.StructureTemplate {
	out := make([]protocol.StructureTemplate, len(defaultCatalog))
	for i, t := range defaultCatalog {
		out[i] = cloneTemplate(t)
	}
	return out
}

func buildBuiltin() []protocol.StructureTemplate {
	sp := spiritIndex
	seat := func(id, title, rank, spirit, manager, model string, lead bool) protocol.StructureSeat {
		v := sp[spirit]
		return protocol.StructureSeat{ID: id, Title: title, Rank: rank, DefaultSpirit: spirit, ReportsToSeat: manager, Lead: lead, Duty: v.RoleMapping, CapabilitiesHint: v.RoleMapping, SuggestedModelClass: model}
	}
	makeTemplate := func(id, name, culture, shape, reporting, description string, jobs []string, min, max, def int, seats []protocol.StructureSeat) protocol.StructureTemplate {
		return protocol.StructureTemplate{ID: id, Name: name, Culture: culture, Shape: shape, Reporting: reporting, Description: description, JobFit: jobs, MinSeats: min, MaxSeats: max, DefaultSeats: def, Seats: seats, Builtin: true, Revision: 1, Source: taxonomySource}
	}
	court := makeTemplate("goetia-court", "The Goetia Court", "goetia", "deep", protocol.ReportingTree,
		"A sovereign-led Ars Goetia court: the king directs, dukes execute, and specialists review, verify and record. Office-to-role translations never change permissions.",
		[]string{"build", "campaign"}, 6, 9, 7, []protocol.StructureSeat{
			seat("sovereign", "Sovereign", "king", "paimon", "", "smart", true),
			seat("executor-formats", "Executor", "duke", "barbatos", "sovereign", "fast-tool-capable", false),
			seat("executor-build", "Executor", "duke", "eligos", "sovereign", "fast-tool-capable", false),
			seat("specialist", "Specialist", "marquis", "naberius", "executor-build", "specialist", false),
			seat("verifier", "Verifier", "earl", "andromalius", "executor-formats", "reasoning", false),
			seat("recorder", "Recorder", "president", "marbas", "sovereign", "fast", false),
			seat("treasurer", "Treasurer", "duke", "bune", "sovereign", "reasoning", false),
			seat("lone-operative", "Lone operative", "knight", "furcas", "sovereign", "specialist", false),
			seat("executor-reserve", "Executor", "duke", "barbatos", "sovereign", "fast-tool-capable", false),
		})
	court.Source = goetiaSource
	court.Notes = []string{"The ninth seat repeats Barbatos as an independently identified Duke executor, not a new spirit.", "Canonical ranks retain dual ranks. Gaap's prince/president identity is distinct from the separate chief-kings classification; role titles are application mappings, not permission levels."}
	princes := makeTemplate("seven-princes", "The Seven Princes", "binsfeld", "wide", protocol.ReportingPeers,
		"Binsfeld's seven deadly-sin associations become seven parallel, honest lenses; an optional synthesizer preserves the strongest dissent.", []string{"research", "decide"}, 7, 8, 8, nil)
	for _, id := range []string{"lucifer", "mammon", "asmodeus", "leviathan", "beelzebub", "satan", "belphegor"} {
		princes.Seats = append(princes.Seats, seat(id, sp[id].Name, sp[id].Ranks[0], id, "", "reasoning", false))
	}
	princes.Seats = append(princes.Seats, protocol.StructureSeat{ID: "synthesizer", Title: "Synthesizer", Rank: "synthesizer", Duty: "Weigh all seven lenses, preserve the strongest dissent, and return a recommendation to the Summoner.", SuggestedModelClass: "smart", Lead: true})
	ring := makeTemplate("elemental-ring", "The Elemental Ring", "elemental", "ring", protocol.ReportingPeers,
		"Four cross-cultural domain peers guard action, preservation, flow and messages. Optional Paracelsian earth guardian adds infrastructure stability; no peer gains permission from its element.", []string{"watch", "guard", "operations"}, 4, 5, 4, []protocol.StructureSeat{
			seat("fire", "Fire guardian", "fire", "djinn", "", "fast-tool-capable", false),
			seat("ice", "Ice guardian", "ice", "jotunn", "", "reasoning", false),
			seat("water", "Water guardian", "water", "kappa", "", "fast-tool-capable", false),
			seat("wind", "Wind guardian", "wind", "pazuzu", "", "fast", false),
			seat("earth", "Earth guardian", "earth", "gnome", "", "reasoning", false),
		})
	ring.Notes = []string{sp["gnome"].Notes, "The faithful default is the preserved taxonomy's four named fire/ice/water/wind figures. Earth is optional, not a mandatory or inferred HellHorror assignment."}
	parade := makeTemplate("night-parade", "The Night Parade", "japanese", "horde", protocol.ReportingTree,
		"Hyakki Yagyō (百鬼夜行), the Night Parade of One Hundred Demons, inspires a variable procession: Oni marshals distinct specialists, sized to the work.", []string{"triage", "parallel", "batch"}, 2, protocol.TeamsMembersMax, 9, []protocol.StructureSeat{
			seat("marshal", "Marshal", "marshal", "oni", "", "smart", true),
			seat("kitsune", "Adaptive specialist", "horde", "kitsune", "marshal", "fast-tool-capable", false),
			seat("tengu", "Guardian and router", "horde", "tengu", "marshal", "fast", false),
			seat("kappa", "Data specialist", "horde", "kappa", "marshal", "fast-tool-capable", false),
			seat("tanuki", "Transformation specialist", "horde", "tanuki", "marshal", "specialist", false),
			seat("bakeneko", "Quick classifier", "horde", "bakeneko", "marshal", "fast", false),
			seat("hitodama", "Memory specialist", "horde", "hitodama", "marshal", "reasoning", false),
			seat("mokumokuren", "Visual inspector", "horde", "mokumokuren", "marshal", "vision", false),
		})
	parade.Variable = true
	parade.Source = "Matthew Meyer, The Night Parade of One Hundred Demons: https://yokai.com/product/the-night-parade-of-one-hundred-demons/; " + yokaiSource
	parade.Notes = []string{"All counts include Oni. The worked example has eight specialists plus its marshal: nine total seats."}
	orders := makeTemplate("nine-orders", "The Nine Orders", "despina", "court", protocol.ReportingPeers,
		"HellHorror's nine-label secondary taxonomy becomes a quality court. Its attribution to Alphonsus de Spina is not established from a primary text here; retained labels and application duties are an explicit adaptation.", []string{"verify", "harden", "quality"}, 1, 9, 9, nil)
	for _, id := range []string{"inquisitors", "accusers", "furies", "tempters", "vengeful", "creators-of-tempests", "deluding", "lying-spirits", "false-gods"} {
		orders.Seats = append(orders.Seats, seat(id, sp[id].Name, id, id, "", "reasoning", false))
	}
	orders.Notes = []string{"These are application role labels from HellHorror, not a verified De Spina historical list. Agrippa's Three Books of Occult Philosophy III.xviii instead names vessels of iniquity as the third order and accusers/inquisitors together as the eighth: https://esotericarchives.com/agrippa/agripp3b.htm"}
	council := makeTemplate("pagan-council", "The Pagan Council", "pagan", "wide", protocol.ReportingPeers,
		"A council of distinct mythic voices debates creative and advisory choices. These figures come from living and historical traditions; outsider demon classifications are not religious claims.", []string{"decide", "create", "advise"}, 5, 7, 6, nil)
	for _, id := range []string{"hecate", "kali", "pan", "loki", "lilith", "baphomet"} {
		council.Seats = append(council.Seats, seat(id, sp[id].Name, "council", id, "", "reasoning", false))
	}
	council.Seats = append(council.Seats, protocol.StructureSeat{ID: "synthesizer", Title: "Synthesizer", Rank: "synthesizer", Duty: "Synthesize the council's advice and preserve dissent; the Summoner decides.", SuggestedModelClass: "smart", Lead: true})
	council.Notes = []string{sp["kali"].Notes, "Users may clone and rename/remove any figure. Hecate corrects the source spelling Hectate; Baphomet is an occult image, not a claim of a historical pagan deity."}
	out := []protocol.StructureTemplate{court, princes, ring, parade, orders, council}
	for i := range out {
		out[i].Example = workedExample(out[i])
	}
	return out
}

// Lookup resolves only shipped defaults; use Store.List for local templates and
// visibility. It returns a fresh value and never opens account/provider state.
func Lookup(id string) (protocol.StructureTemplate, bool) {
	for _, t := range defaultCatalog {
		if t.ID == id {
			return cloneTemplate(t), true
		}
	}
	return protocol.StructureTemplate{}, false
}
