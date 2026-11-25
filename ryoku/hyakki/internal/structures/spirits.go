// Package structures supplies local, account-free Yokai recipes. It does not
// execute agents or grant tools, model access, approvals, or budget.
package structures

import (
	"sort"
	"strings"

	"ryoku-rashin/internal/protocol"
)

const goetiaSource = "Joseph H. Peterson, Lemegeton, Part 1: Goetia, Esoteric Archives: https://www.esotericarchives.com/solomon/goetia.htm"
const taxonomySource = "HellHorror, Types of Demons / Ranks with Demon Names (secondary taxonomy): https://hellhorror.com/types-of-demons"
const yokaiSource = "Matthew Meyer, Yokai.com, The Illustrated Database of Japanese Folklore (modern folklore introduction): https://yokai.com/"

var canonicalSpirits = buildSpirits()
var spiritIndex = func() map[string]protocol.StructureSpirit {
	index := make(map[string]protocol.StructureSpirit, len(canonicalSpirits))
	for _, s := range canonicalSpirits {
		index[s.ID] = s
	}
	return index
}()

// Spirits returns independent copies of all 72 Goetia identities and the other
// catalog identities. Source spellings remain names, while IDs are stable keys.
func Spirits() []protocol.StructureSpirit {
	out := make([]protocol.StructureSpirit, len(canonicalSpirits))
	for i, s := range canonicalSpirits {
		out[i] = s
		out[i].Ranks = append([]string(nil), s.Ranks...)
	}
	return out
}

func buildSpirits() []protocol.StructureSpirit {
	byID := make(map[string]protocol.StructureSpirit)
	groups := []struct{ rank, names string }{
		{"king", "Bael,Paimon,Beleth,Purson,Asmoday,Vine,Balam,Zagan,Belial"},
		{"duke", "Agares,Valefar,Barbatos,Gusoin,Eligor,Zepar,Bathin,Saleos,Aim,Bune,Berith,Astaroth,Focalor,Vepar,Vual,Procel,Alloces,Murmur,Gemory,Vapula,Flauros,Amduscias,Dantalion"},
		{"marquis", "Gamigin,Amon,Leraye,Naberius,Ronove,Forneus,Marchosias,Phoenix,Sabnach,Shax,Orias,Andras,Andrealphus,Cimeies,Decarabia"},
		{"earl", "Botis,Morax,Ipos,Ronove,Furfur,Halphas,Raum,Vine,Bifrons,Murmur,Andromalius"},
		{"prince", "Vassago,Sitri,Ipos,Gaap,Stolas,Orobas,Seere"},
		{"knight", "Furcas"},
		{"president", "Marbas,Buer,Botis,Morax,Glasya Labolas,Foras,Gaap,Malphas,Haagenti,Caim,Ose,Amy,Zagan,Valac"},
	}
	for _, group := range groups {
		for _, name := range strings.Split(group.names, ",") {
			id := strings.ReplaceAll(strings.ToLower(name), " ", "-")
			v := byID[id]
			v.ID, v.Name, v.Culture, v.Source = id, name, "goetia", goetiaSource
			v.Ranks = append(v.Ranks, group.rank)
			byID[id] = v
		}
	}
	// Eligos is the catalog spelling of Peterson's Eligor, not a 73rd spirit.
	v := byID["eligor"]
	delete(byID, "eligor")
	v.ID, v.Name, v.Notes = "eligos", "Eligos", "Peterson spells this identity Eligor; the catalog uses Eligos."
	byID[v.ID] = v
	offices := map[string][2]string{
		"paimon":      {"Teaches all arts and sciences, reveals hidden things, gives dignity; very obedient to Lucifer.", "Sets direction, holds the goal and synthesizes the final decision."},
		"barbatos":    {"Understands the speech of birds and beasts, reveals hidden treasure, knows past and future.", "Parses foreign formats and protocols; uncovers code coupling."},
		"eligos":      {"Discovers hidden things, knows war and the future, gains favor of lords.", "Investigates, implements, surfaces blockers and secures stakeholder understanding."},
		"naberius":    {"Restores lost dignities and honors; teaches arts and sciences.", "Craft specialist who reviews design, security and implementation quality."},
		"andromalius": {"Discovers theft and the thief, recovers what was stolen, punishes wrongdoing.", "Audits leaks and regressions, recovers lost work and demands evidence."},
		"marbas":      {"Answers truly on hidden things, causes and cures disease, changes shape and gives mechanical wisdom.", "Records reports and memory; explains what happened."},
		"bune":        {"Gives riches, eloquence and wisdom, answers truly and removes dead bodies.", "Tracks budget and makes the team's output persuasive."},
		"furcas":      {"Teaches philosophy, astronomy, rhetoric, logic, chiromancy and pyromancy.", "Handles one bounded, erudite specialist task."},
	}
	for id, pair := range offices {
		v := byID[id]
		v.Office, v.RoleMapping = pair[0], pair[1]
		byID[id] = v
	}
	v = byID["glasya-labolas"]
	v.Notes = "The preserved Peterson classification lists President only; HellHorror lists Count/President. This catalog follows Peterson's per-spirit entries, not conflicting headings or counts."
	byID[v.ID] = v
	add := func(id, name, culture, rank, office, role, source, notes string) {
		byID[id] = protocol.StructureSpirit{ID: id, Name: name, Culture: culture, Ranks: []string{rank}, Office: office, RoleMapping: role, Source: source, Notes: notes}
	}
	for _, p := range []struct{ id, name, rank, role string }{
		{"lucifer", "Lucifer", "pride", "Ambition and the big picture: what would make this great?"},
		{"mammon", "Mammon", "greed", "Resources, gain and cost."},
		{"asmodeus", "Asmodeus", "lust", "What users actually desire."},
		{"leviathan", "Leviathan", "envy", "Comparison and competitors: what do others do better?"},
		{"beelzebub", "Beelzebub", "gluttony", "Gather every relevant signal without omitting evidence."},
		{"satan", "Satan", "wrath", "Adversarial review: what breaks and what is wrong?"},
		{"belphegor", "Belphegor", "sloth", "The least-effort effective solution."},
	} {
		add(p.id, p.name, "binsfeld", p.rank, "Binsfeld's 1589 association: "+p.rank+".", p.role, taxonomySource+" §Binsfeld", "This is Binsfeld's particular scheme, not a universal classification. Asmodeus here and Asmoday in the Goetia are tradition-specific catalog records.")
	}
	for _, p := range []struct{ id, name, culture, rank, office, role string }{
		{"djinn", "Djinn", "arabian", "fire", "Fire-beings of Arabian lore.", "Builds and executes only approved actions."},
		{"jotunn", "Jotunn", "norse", "ice", "Norse giants associated here with ice and stone.", "Preserves backups, archives and state."},
		{"kappa", "Kappa", "japanese", "water", "Japanese water spirit.", "Moves data and checks synchronization."},
		{"pazuzu", "Pazuzu", "mesopotamian", "wind", "Mesopotamian wind demon; king of the demons of the wind.", "Carries messages and timely alerts."},
		{"gnome", "Gnome", "elemental", "earth", "Guards the treasures of the earth in Paracelsus's account.", "Guards infrastructure, storage capacity and stability."},
		{"oni", "Oni", "japanese", "marshal", "Japanese oni, an ogre-like figure in folklore.", "Holds the goal and marshals the specialists."},
		{"kitsune", "Kitsune", "japanese", "horde", "Fox yōkai associated with shapeshifting.", "Adaptive planning and coding."},
		{"tengu", "Tengu", "japanese", "horde", "Japanese mountain yōkai; traditions distinguish greater and lesser tengu.", "Guards, watches and routes urgent work."},
		{"tanuki", "Tanuki", "japanese", "horde", "Shapeshifting tanuki of Japanese folklore.", "Transforms and rewrites."},
		{"bakeneko", "Bakeneko", "japanese", "horde", "Cat spirit of Japanese folklore.", "Quick classification and reflex tasks."},
		{"hitodama", "Hitodama", "japanese", "horde", "Soul-flame of Japanese folklore.", "Memory and recall."},
		{"mokumokuren", "Mokumokuren", "japanese", "horde", "Many-eyed yōkai; the name means many eyes.", "Visual and screen inspection."},
		{"hecate", "Hecate", "greek", "council", "Crossroads, magic, night and liminal spaces.", "Maps decision points and paths not taken."},
		{"kali", "Kali", "hindu", "council", "Time, transformation and destruction of the false.", "Identifies what must be cut or transformed."},
		{"pan", "Pan", "greek", "council", "Wilds, nature, instinct and the untamed.", "Asks what is natural and organic for users."},
		{"loki", "Loki", "norse", "council", "Trickery, change and shape-shifting.", "Finds unconventional alternatives."},
		{"lilith", "Lilith", "mesopotamian-jewish", "council", "Night, independence and refusal to submit in folklore.", "Challenges groupthink independently."},
		{"baphomet", "Baphomet", "occult", "council", "Balance of opposites in the occult image associated with Éliphas Lévi.", "Balances tensions without erasing dissent."},
	} {
		notes := ""
		if p.id == "gnome" {
			notes = "Optional Paracelsian earth guardian, not an elemental assignment supplied by HellHorror. Paracelsus, A Book on Nymphs, Sylphs, Pygmies, and Salamanders, Tractatus VI (1941 translation): 'gnomes, pygmies and mami guard the treasures of the earth'. Primary text: https://archive.org/stream/fourtreatisesoft00para/fourtreatisesoft00para_djvu.txt"
		}
		if p.id == "kali" {
			notes = "Kali is a revered goddess in living Hindu tradition, not a demon. The reference's outsider classification is not a religious claim; this is a respectful mythic role."
		}
		if p.id == "hecate" {
			notes = "The preserved taxonomy misspells Hecate as Hectate; the catalog uses Hecate."
		}
		source := taxonomySource
		if p.id == "gnome" {
			source = "Paracelsus, A Book on Nymphs, Sylphs, Pygmies, and Salamanders, Tractatus VI, in Four Treatises (Johns Hopkins Press, 1941): https://archive.org/stream/fourtreatisesoft00para/fourtreatisesoft00para_djvu.txt"
		}
		add(p.id, p.name, p.culture, p.rank, p.office, p.role, source, notes)
		switch p.id {
		case "oni", "kitsune", "tengu", "tanuki", "bakeneko", "hitodama", "mokumokuren":
			entry := byID[p.id]
			entry.Ranks = nil
			page := p.id
			if p.id == "tengu" {
				page = "daitengu"
			}
			entry.Source = yokaiSource + "; " + p.name + " identity: https://yokai.com/" + page + "/"
			entry.Notes = "Folklore varies between traditions. No canonical hierarchy rank is asserted here; marshal/horde labels and agent duties are Hyakki role mappings, not folklore offices or permission grants."
			byID[p.id] = entry
		}
	}
	for _, p := range []struct{ id, name, role string }{
		{"false-gods", "False gods", "Challenges false premises and unsupported authority claims."},
		{"lying-spirits", "Lying spirits", "Fact-checks falsehoods; the duty opposes the type's name."},
		{"inquisitors", "Inquisitors", "Investigates, queries and establishes facts."},
		{"vengeful", "Vengeful", "Hunts regressions and restores broken behavior."},
		{"deluding", "Deluding", "Detects hallucination and misleading output."},
		{"creators-of-tempests", "Creators of Tempests", "Stress-tests, loads and fuzzes within approved boundaries."},
		{"furies", "Furies", "Applies agreed fixes and corrections."},
		{"accusers", "Accusers", "Critiques suspected defects and demands evidence."},
		{"tempters", "Tempters", "Writes accurate, compelling output."},
	} {
		add(p.id, p.name, "despina", p.id, "One of the nine functional labels in HellHorror's secondary taxonomy.", p.role, taxonomySource+"; compare Heinrich Cornelius Agrippa, Three Books of Occult Philosophy III.xviii: https://esotericarchives.com/agrippa/agripp3b.htm", "HellHorror attributes these labels to Alphonsus de Spina; that primary attribution is not established here. Agrippa's taxonomy differs, including vessels of iniquity as the third order and accusers/inquisitors as the eighth. These retained application roles are an adaptation, not a verified historical list or invented individual spirits.")
	}
	out := make([]protocol.StructureSpirit, 0, len(byID))
	for _, v := range byID {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
