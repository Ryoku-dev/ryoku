package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBetterOffServed_FixByBuffering(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Chatty traffic is better off served by the AFD buffering.", NewBetterOffServed(), "Chatty traffic is better served by the AFD buffering.")
}

func TestBetterOffServed_FixIfFalse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "IMHO, most use cases would be far better off served if drag-content=false would imply that the user would still be able drag the main content.", NewBetterOffServed(), "IMHO, most use cases would be far better off if drag-content=false would imply that the user would still be able drag the main content.")
}

func TestBetterOffServed_FixOverTls(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This website is serving a curl|sh command and probably better off served over TLS with HSTS/cert pinning.", NewBetterOffServed(), "This website is serving a curl|sh command and probably better served over TLS with HSTS/cert pinning.")
}

func TestBetterOffServed_FixContributing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Don't start it if your code would be better off served contributing to another project where you could ...", NewBetterOffServed(), "Don't start it if your code would be better off contributing to another project where you could ...")
}

func TestBetterOffServed_FixNotUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "IMHO you are also better off served NOT using a single frame and trying to shove everything into it.", NewBetterOffServed(), "IMHO you are also better off NOT using a single frame and trying to shove everything into it.")
}

func TestBetterOffServed_FixJustDoing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SSR is neat but I feel like you're better off served just doing it with something else", NewBetterOffServed(), "SSR is neat but I feel like you're better off just doing it with something else")
}

func TestBetterOffServed_FixAsAComebackMechanic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Personally I think the bonus recruitments would be better off served as a comeback mechanic that triggers when ...", NewBetterOffServed(), "Personally I think the bonus recruitments would be better served as a comeback mechanic that triggers when ...")
}

func TestBetterOffServed_FixWithADifferentTool(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The second thing you are asking about is likely better off served with a different tool.", NewBetterOffServed(), "The second thing you are asking about is likely better served with a different tool.")
}

func TestBetterOffServed_FixPlanningAndExecuting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You are better off served planning and executing some meaningful training.", NewBetterOffServed(), "You are better off planning and executing some meaningful training.")
}

func TestBetterOffServed_IgnoreServedCold(t *testing.T) {
	linttest.AssertNoLints(t, "All movies are better off served cold.", NewBetterOffServed())
}

func TestBetterOffServed_FixMakingAutomod(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You might be better off served by making Automod hold all posts and/or comments.", NewBetterOffServed(), "You might be better served by making Automod hold all posts and/or comments.")
}

func TestBetterOffServed_FixLookingInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Given the high consistency of the negative feedback, I'd say you're better off served looking into putting that cash into another game ...", NewBetterOffServed(), "Given the high consistency of the negative feedback, I'd say you're better off looking into putting that cash into another game ...")
}

func TestBetterOffServed_FixWithDevices(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Otherwise, you're better off served with specialist devices.", NewBetterOffServed(), "Otherwise, you're better served with specialist devices.")
}

func TestBetterOffServed_FixByTurning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However, once they are learned, we are better off served by turning our focus forward.", NewBetterOffServed(), "However, once they are learned, we are better served by turning our focus forward.")
}

func TestBetterOffServed_IgnoreSlash(t *testing.T) {
	linttest.AssertNoLints(t, "This company is better off/served without you.", NewBetterOffServed())
}

func TestBetterOffServed_FixInAView(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If so that doesn't belong in the table, and would be better off served in a VIEW .", NewBetterOffServed(), "If so that doesn't belong in the table, and would be better served in a VIEW .")
}
