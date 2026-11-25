package settings

import (
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestPartsSettingsSayWhatUsesAPartWhereItStandsAndWhy(t *testing.T) {
	model := protocol.Component{ID: "model:whisper-base.en-q5_1", Kind: protocol.ComponentModel, Title: "Speech model whisper-base.en-q5_1",
		State: protocol.ComponentPresent, Bytes: 59721011, Removable: true, Uses: "Talking to Oni and dictation, through whisper.cpp."}
	if got := partDetail(model); !strings.Contains(got, "Talking to Oni") || !strings.Contains(got, "On this computer, 57.0 MB") {
		t.Fatalf("downloaded model reads %q", got)
	}
	if !partRemovable(model) {
		t.Fatal("a downloaded model offers no Remove")
	}
	inUse := model
	inUse.InUse = "Oni listens with this model while voice is on. Pick another model or turn voice off first."
	if got := partDetail(inUse); !strings.HasSuffix(got, inUse.InUse) {
		t.Fatalf("a model in use does not say why it stays: %q", got)
	}
	absent := model
	absent.State, absent.Bytes = protocol.ComponentAbsent, 0
	if partRemovable(absent) || !strings.Contains(partDetail(absent), "Not on this computer") {
		t.Fatalf("absent model: removable %v, %q", partRemovable(absent), partDetail(absent))
	}
	kept := model
	kept.Removable, kept.Why = false, "Ryoku's dictation keeps this model; Hyakki uses it where it is and never removes it."
	if partRemovable(kept) || !strings.Contains(partDetail(kept), kept.Why) {
		t.Fatalf("Ryoku's model: removable %v, %q", partRemovable(kept), partDetail(kept))
	}
	partial := model
	partial.State, partial.Bytes = protocol.ComponentPartial, 2048
	if got := partDetail(partial); !strings.Contains(got, "Partly downloaded, 2 KB") || !partRemovable(partial) {
		t.Fatalf("partial download reads %q", got)
	}
}

func TestPartsSettingsKeepProgramsApartAndNeverRemovable(t *testing.T) {
	components := []protocol.Component{
		{ID: "pack:kokoro", Kind: protocol.ComponentPack, Title: "Kokoro voice pack", State: protocol.ComponentPresent, Bytes: 3 << 30, Removable: true},
		{ID: "program:tesseract", Kind: protocol.ComponentProgram, Title: "Tesseract text reader", State: protocol.ComponentAbsent, Needs: []string{"tesseract"},
			Why: "Reads the words in a window so a guide can point at them."},
		{ID: "program:grim", Kind: protocol.ComponentProgram, Title: "grim screenshots", State: protocol.ComponentPresent, Path: "/usr/bin/grim", Removable: true},
		{ID: "program:decision-server", Kind: protocol.ComponentProgram, Title: "Local decision server", State: protocol.ComponentUnknown},
		{ID: "data:reminders", Kind: protocol.ComponentData, Title: "Reminders and timers", State: protocol.ComponentAbsent, Removable: true},
	}
	owned, programs := partSplit(components)
	if len(owned) != 2 || owned[0].ID != "pack:kokoro" || owned[1].ID != "data:reminders" || len(programs) != 3 {
		t.Fatalf("owned %+v, programs %+v", owned, programs)
	}
	for _, c := range programs {
		if partRemovable(c) {
			t.Fatalf("program %s offers Remove", c.ID)
		}
	}
	for c, want := range map[int]string{0: "Not installed: tesseract", 1: "Installed: /usr/bin/grim", 2: "Not looked for"} {
		if got := partDetail(programs[c]); !strings.Contains(got, want) {
			t.Fatalf("%s reads %q, want %q", programs[c].ID, got, want)
		}
	}
	if got := partsSummary(owned, ""); got != "1 on this computer, 3.0 GB in all." {
		t.Fatalf("summary %q", got)
	}
	if got := partsSummary(nil, ""); got != "None of them is on this computer." {
		t.Fatalf("empty summary %q", got)
	}
	if got := partConfirmBody(owned[0]); !strings.Contains(got, "3.0 GB") || !strings.Contains(got, "Nothing outside Hyakki's own folders") {
		t.Fatalf("question body %q", got)
	}
}
