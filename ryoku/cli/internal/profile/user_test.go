package profile

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestInstallProfileStoreItemsReportsWarnings(t *testing.T) {
	original := profileStoreInstall
	t.Cleanup(func() { profileStoreInstall = original })

	var calls []StoreItem
	profileStoreInstall = func(category, id string) error {
		calls = append(calls, StoreItem{Category: category, ID: id})
		if id == "python" {
			return errors.New("catalogue unavailable")
		}
		return nil
	}
	items := []StoreItem{
		{Category: "barstyles", ID: "qsbar"},
		{Category: "subshells", ID: "python"},
	}
	warnings := installProfileStoreItems(items)
	if !reflect.DeepEqual(calls, items) {
		t.Fatalf("install calls = %#v, want %#v", calls, items)
	}
	if len(warnings) != 1 || warnings[0].Kind != "storeItem" || !strings.Contains(warnings[0].Detail, "subshells/python") {
		t.Fatalf("warnings = %#v", warnings)
	}
}
