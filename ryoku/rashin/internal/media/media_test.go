package media

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestVTTTimestampedRollingCaptions(t *testing.T) {
	vtt := "\ufeffWEBVTT\r\n\r\nKind: captions\r\n\r\n1\r\n00:00:01.200 --> 00:00:03.000 align:start position:0%\r\n<c>Hello &amp; welcome</c>\r\n\r\n2\r\n00:00:02.400 --> 00:00:04.000\r\nHello &amp; welcome\r\n\r\n3\r\n00:00:03.000 --> 00:00:06.000\r\nHello &amp; welcome\r\nto <00:00:03.200><c>the talk</c>\r\n\r\n4\r\n00:00:05.000 --> 00:00:08.000\r\nto the talk today\r\n\r\n5\r\n01:02:03.000 --> 01:02:04.000\r\nGoodbye\r\n"
	want := "[00:01] Hello & welcome\n[00:03] to the talk\n[00:05] today\n[62:03] Goodbye"
	if got := VTTText(vtt); got != want {
		t.Fatalf("captions = %q, want %q", got, want)
	}
}

func TestVTTTwoPartTimesAndMalformedCues(t *testing.T) {
	vtt := "WEBVTT\n\nNOTE a note\n\n00:02.500 --> 00:04.000\nA line\n\nnot-a-time --> 00:10.000\nNot a cue\n\n00:99.000 --> 01:40.000\nInvalid seconds\n\nNaN:01 --> 00:04.000\nInvalid number\n\n00:07.000 --> 00:09.000\nA new line\n"
	if got := VTTText(vtt); got != "[00:02] A line\n[00:07] A new line" {
		t.Fatalf("captions = %q", got)
	}
	if VTTText("WEBVTT\n\n") != "" {
		t.Fatal("empty captions produced text")
	}
}

func TestMessageURLs(t *testing.T) {
	text := "Summarize [this](https://www.youtube.com/watch?v=abc&t=12), https://youtu.be/def. Then <https://vimeo.com/123> and https://example.org/watch_(part_1). https://youtu.be/def is repeated, https://user:secret@example.org/private must not be used"
	want := []string{"https://www.youtube.com/watch?v=abc&t=12", "https://youtu.be/def", "https://vimeo.com/123", "https://example.org/watch_(part_1)"}
	if got := URLs(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("URLs = %#v, want %#v", got, want)
	}
	for _, test := range []struct {
		url  string
		want bool
	}{
		{"https://youtube.com/watch?v=x", true},
		{"https://m.youtube.com/watch?v=x", true},
		{"https://YOUTU.BE/x", true},
		{"https://player.vimeo.com/video/1", true},
		{"https://youtube.com.example.org/video", false},
		{"https://notyoutube.com/video", false},
		{"https://example.org/?site=youtube.com", false},
	} {
		if got := KnownVideoURL(test.url); got != test.want {
			t.Errorf("KnownVideoURL(%q) = %v", test.url, got)
		}
	}
}

func TestTranscriptLimitCountsCharacters(t *testing.T) {
	text := strings.Repeat("界", 60001)
	got := LimitTranscript(text)
	prefix, note, found := strings.Cut(got, "\n[")
	if !found || utf8.RuneCountInString(prefix) != 60000 || !utf8.ValidString(got) || !strings.Contains(note, "shortened") {
		t.Fatalf("invalid truncation: %d characters, note %q", utf8.RuneCountInString(prefix), note)
	}
	short := strings.Repeat("界", 60000)
	if LimitTranscript(short) != short {
		t.Fatal("transcript exactly at the limit was changed")
	}
}
