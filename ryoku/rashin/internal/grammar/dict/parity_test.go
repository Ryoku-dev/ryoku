package dict

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCuratedBudget(t *testing.T) {
	if os.Getenv("HARPER_BUDGET") == "" {
		t.Skip("opt-in cold dictionary budget measurement")
	}
	debug.FreeOSMemory()
	baselineRSS := residentBytes(t)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	d := Curated()
	elapsed := time.Since(start)
	unscavengedRSS := residentBytes(t)
	debug.FreeOSMemory()
	runtime.ReadMemStats(&after)
	rss := residentBytes(t)
	t.Logf("Curated words=%d cold=%s retained_heap=%d bytes resident_added=%d bytes settled_RSS=%d bytes unscavenged_RSS=%d bytes", d.WordCount(), elapsed, int64(after.HeapAlloc)-int64(before.HeapAlloc), rss-baselineRSS, rss, unscavengedRSS)
	if rss-baselineRSS >= 60_000_000 {
		t.Errorf("resident dictionary exceeds 60MB: %d bytes", rss-baselineRSS)
	}
	if elapsed > 250*time.Millisecond {
		t.Errorf("cold build exceeds 250ms: %s", elapsed)
	}
}
func residentBytes(t *testing.T) int64 {
	t.Helper()
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			return kb * 1024
		}
	}
	t.Fatal("VmRSS missing")
	return 0
}
func TestOracleWordSetAndMetadata(t *testing.T) {
	oracle := os.Getenv("HARPER_ORACLE")
	if oracle == "" {
		t.Skip("set HARPER_ORACLE for upstream parity")
	}
	raw, err := exec.Command(oracle, "words").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for i, w := range want {
		want[i], err = strconv.Unquote(w)
		if err != nil {
			t.Fatal(err)
		}
	}
	got := make([]string, 0, Curated().WordCount())
	for _, word := range Curated().WordsIter() {
		got = append(got, string(word))
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(want, got) {
		expected := map[string]bool{}
		actual := map[string]bool{}
		for _, w := range want {
			expected[w] = true
		}
		for _, w := range got {
			actual[w] = true
		}
		var missing, extra []string
		for w := range expected {
			if !actual[w] {
				missing = append(missing, w)
			}
		}
		for w := range actual {
			if !expected[w] {
				extra = append(extra, w)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)
		t.Fatalf("word sets differ: oracle=%d Go=%d missing(%d)=%v extra(%d)=%v", len(want), len(got), len(missing), missing[:min(30, len(missing))], len(extra), extra[:min(30, len(extra))])
	}
	count := 0
	for i := 0; i < len(want); i += max(1, len(want)/400) {
		w := want[i]
		raw, err := exec.Command(oracle, "metadata", w).Output()
		if err != nil {
			t.Fatal(err)
		}
		start := bytes.IndexByte(raw, '{')
		if start < 0 {
			t.Fatalf("no metadata: %s", raw)
		}
		var expected DictWordMetadata
		if err := json.Unmarshal(raw[start:], &expected); err != nil {
			t.Fatal(err)
		}
		actual := Curated().GetWordMetadataStr(w)
		if actual == nil || !actual.Equal(expected) {
			b, _ := json.Marshal(actual)
			t.Fatalf("metadata mismatch %q\nwant %s\ngot %s", w, raw[start:], b)
		}
		count++
	}
	t.Logf("word set exact: %d; metadata exact: %d samples", len(got), count)
}
func TestOracleSuggestionOrder(t *testing.T) {
	oracle := os.Getenv("HARPER_ORACLE")
	if oracle == "" {
		t.Skip("set HARPER_ORACLE for upstream suggestion parity")
	}
	words := Curated().WordsIter()
	sort.Slice(words, func(i, j int) bool { return runeLess(words[i], words[j]) })
	var input strings.Builder
	seen := map[string]bool{}
	count := 0
	for i := 97; count < 420; i = (i + 337) % len(words) {
		w := words[i]
		valid := len(w) >= 5 && len(w) <= 18
		for _, c := range w {
			if c < 'a' || c > 'z' {
				valid = false
			}
		}
		if !valid {
			continue
		}
		typo := append([]rune(nil), w...)
		j := len(typo) / 2
		switch count % 4 {
		case 0:
			typo = append(typo[:j], typo[j+1:]...)
		case 1:
			typo[j] = 'z'
		case 2:
			typo[j], typo[j+1] = typo[j+1], typo[j]
		case 3:
			typo[j] = 'x'
			typo[j-1] = 'q'
		}
		word := string(typo)
		if Curated().ContainsWord(typo) || seen[word] {
			continue
		}
		seen[word] = true
		count++
		input.WriteString(word)
		input.WriteString(".\n")
	}
	path := t.TempDir() + "/misspellings.txt"
	if err := os.WriteFile(path, []byte(input.String()), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.Command(oracle, "lint", "--format", "json", "--quiet", "--dialect", "us", path).Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			t.Fatal(err)
		}
	}
	var files []struct {
		Lints []struct {
			Rule        string   `json:"rule"`
			MatchedText string   `json:"matched_text"`
			Suggestions []string `json:"suggestions"`
		} `json:"lints"`
	}
	if err := json.Unmarshal(raw, &files); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	matched, total := 0, 0
	for _, file := range files {
		for _, l := range file.Lints {
			if l.Rule != "SpellCheck" {
				continue
			}
			total++
			var actual []string
			for distance := uint8(2); distance < 5; distance++ {
				dedup := map[string]bool{}
				for _, s := range SuggestCorrectSpellingStr(l.MatchedText, 200, distance, Curated()) {
					m := Curated().GetWordMetadataStr(s)
					key := strings.ToLower(s)
					if m != nil && m.Dialects.IsDialectEnabled(American) && !dedup[key] {
						dedup[key] = true
						actual = append(actual, "Replace with: “"+s+"”")
						if len(actual) == 3 {
							break
						}
					}
				}
				if len(actual) > 0 {
					break
				}
			}
			if reflect.DeepEqual(actual, l.Suggestions) {
				matched++
			} else {
				t.Errorf("suggestion mismatch %q want=%v got=%v", l.MatchedText, l.Suggestions, actual)
			}
		}
	}
	if total < 300 {
		t.Errorf("expected >=300 SpellCheck lints, got %d", total)
	}
	t.Logf("misspelling file=%d words; ordered suggestions exact=%d/%d (%.2f%%)", count, matched, total, 100*float64(matched)/float64(total))
}
