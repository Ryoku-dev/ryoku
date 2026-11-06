package profile

import (
	"reflect"
	"testing"
)

func TestParseResolveOutput(t *testing.T) {
	body := "extra nmap 7.95-1 6369872\ncore glibc 2.41-2 1000\n"
	refs, err := parseResolveOutput(body, []string{"nmap"})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].Name != "nmap" || refs[0].Repo != "extra" || refs[0].Download != 6369872 || !refs[0].Target || refs[1].Target {
		t.Fatalf("refs = %#v", refs)
	}
}

func TestParsePacmanInfo(t *testing.T) {
	body := "Repository      : extra\nName            : nmap\nInstalled Size  : 26.53 MiB\n\nRepository      : core\nName            : glibc\nInstalled Size  : 12.00 KiB\n"
	sizes, err := parsePacmanInfo(body)
	if err != nil {
		t.Fatal(err)
	}
	if sizes["nmap"] != 27818721 || sizes["glibc"] != 12*1024 {
		t.Fatalf("sizes = %#v", sizes)
	}
}

func TestParseRequiredBy(t *testing.T) {
	body := "Name            : libone\nRequired By     : app-one\n                  app-two\n\nName            : leaf\nRequired By     : None\n"
	got := parseRequiredBy(body)
	want := map[string][]string{"libone": {"app-one", "app-two"}, "leaf": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("required = %#v", got)
	}
}

func TestParseResolveOutputRejectsUnexpectedText(t *testing.T) {
	if _, err := parseResolveOutput("warning from pacman\n", nil); err == nil {
		t.Fatal("accepted malformed output")
	}
}
