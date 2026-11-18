package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFriendOfMe_CorrectsFriendOfMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Last year a friend of me died unexpectedly (not a close friend).", NewFriendOfMe(), "Last year a friend of mine died unexpectedly (not a close friend).")
}

func TestFriendOfMe_CorrectsFriendOfYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "imagine a friend of you wants to disturb your call, and send you a session-terminate with a wrong SID", NewFriendOfMe(), "imagine a friend of yours wants to disturb your call, and send you a session-terminate with a wrong SID")
}

func TestFriendOfMe_CorrectsFriendOfUs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have denounced a friend of us! You have denounced an enemy of us.", NewFriendOfMe(), "You have denounced a friend of ours! You have denounced an enemy of ours.")
}

func TestFriendOfMe_CorrectsFriendsOfThem(t *testing.T) {
	linttest.AssertSuggestionResult(t, "guest has friend and they see which friends of them are comming", NewFriendOfMe(), "guest has friend and they see which friends of theirs are comming")
}

func TestFriendOfMe_CorrectsFriendOfHim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ah, got it, i thought you may was a friend of him", NewFriendOfMe(), "Ah, got it, i thought you may was a friend of his")
}

func TestFriendOfMe_CorrectsFriendsOfMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "guest has friend and they see which friends of me are comming", NewFriendOfMe(), "guest has friend and they see which friends of mine are comming")
}

func TestFriendOfMe_CorrectsFriendsOfUs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This project was created for friends of us.", NewFriendOfMe(), "This project was created for friends of ours.")
}
