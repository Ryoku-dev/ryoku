package shell

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/protocol"
)

func localAPI(t *testing.T) (*stage, func() protocol.APISettings) {
	t.Helper()
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/api")
	settings := func() protocol.APISettings { return stage.runtime.store.Snapshot().Settings.API }
	if !settings().Enabled {
		t.Fatal("the demo daemon starts with its local API on")
	}
	return stage, settings
}

func (s *stage) copied(control string) string {
	s.t.Helper()
	s.click(control)
	text := ""
	s.await(control+" puts something on the clipboard", func() bool {
		text = s.clipboard()
		return text != ""
	})
	return text
}

func TestTheLocalAPISwitchTurnsItOffAndOn(t *testing.T) {
	stage, settings := localAPI(t)
	stage.click("Serve the local API")
	stage.await("the switch turns the local API off", func() bool { return !settings().Enabled })
	if _, offered := stage.named("Copy the address"); offered {
		t.Fatal("the address is still offered while the local API is off")
	}
	stage.click("Serve the local API")
	stage.await("the switch turns the local API on again", func() bool { return settings().Enabled })
	if _, offered := stage.named("Copy the address"); !offered {
		t.Fatal("the address is not offered while the local API is on")
	}
}

func TestTheLocalAPIHandsOutItsAddressAndKey(t *testing.T) {
	stage, _ := localAPI(t)
	if address := stage.copied("Copy the address"); address != "http://127.0.0.1:3600/v1" {
		t.Fatalf("copied address = %q", address)
	}
	key := stage.copied("Copy the key")
	if !strings.HasPrefix(key, "rashin-") {
		t.Fatalf("copied key = %q", key)
	}
	exports := stage.copied("Copy exports")
	if exports != "export OPENAI_BASE_URL=http://127.0.0.1:3600/v1\nexport OPENAI_API_KEY="+key+"\n" {
		t.Fatalf("copied exports = %q", exports)
	}
}

func TestTheLocalAPIKeyShowsAndHides(t *testing.T) {
	stage, _ := localAPI(t)
	stage.click("Show the key")
	stage.await("the key is shown", func() bool { _, shown := stage.named("Hide the key"); return shown })
	stage.click("Hide the key")
	if _, hidden := stage.named("Show the key"); !hidden {
		t.Fatal("the key did not hide again")
	}
}

func TestANewLocalAPIKeyReplacesTheOldOneOnlyAfterAYes(t *testing.T) {
	stage, _ := localAPI(t)
	old := stage.copied("Copy the key")
	stage.click("New key")
	stage.click("Cancel")
	if again := stage.copied("Copy the key"); again != old {
		t.Fatalf("the key changed without a yes: %q then %q", old, again)
	}
	stage.click("New key")
	stage.click("Make a new key")
	// the new key is shown at once, so its owner sees that it changed
	stage.await("the new key is shown", func() bool { _, shown := stage.named("Hide the key"); return shown })
	if renewed := stage.copied("Copy the key"); renewed == old {
		t.Fatalf("the key is still %q after a new one was asked for", old)
	}
}

func TestTheLocalAPIPortAppliesOnEnterAndRefusesAPortItCannotUse(t *testing.T) {
	stage, settings := localAPI(t)
	stage.click("Port")
	stage.typeOver(4, "4100")
	if settings().Port != protocol.DefaultAPIPort {
		t.Fatalf("the port changed to %d while it was still being typed", settings().Port)
	}
	stage.press(key.NameReturn, 0)
	stage.await("Enter applies the port", func() bool { return settings().Port == 4100 })
	if address := stage.copied("Copy the address"); address != "http://127.0.0.1:4100/v1" {
		t.Fatalf("the address after the port changed = %q", address)
	}
	stage.click("Port")
	stage.typeOver(4, "80")
	stage.press(key.NameReturn, 0)
	stage.settle()
	if settings().Port != 4100 {
		t.Fatalf("port 80 was applied: %d", settings().Port)
	}
}
