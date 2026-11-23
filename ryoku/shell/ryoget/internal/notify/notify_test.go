package notify

import (
	"slices"
	"testing"
)

func keys(actions []string) []string {
	var out []string
	for i := 0; i < len(actions); i += 2 {
		out = append(out, actions[i])
	}
	return out
}

func TestActionsFollowTheNotice(t *testing.T) {
	if got := Actions(Notice{Title: "Download finished"}); got != nil {
		t.Fatalf("a notice with nothing to act on offers %v", got)
	}
	if got := keys(Actions(Notice{Path: "/d/a.bin", Dir: "/d"})); !slices.Equal(got, []string{ActOpen, ActFolder}) {
		t.Fatalf("finished notice keys = %v", got)
	}
	got := Actions(Notice{Retry: "abc"})
	if !slices.Equal(keys(got), []string{ActRetry}) || len(got) != 2 || got[1] == "" {
		t.Fatalf("failed notice actions = %v", got)
	}
}
