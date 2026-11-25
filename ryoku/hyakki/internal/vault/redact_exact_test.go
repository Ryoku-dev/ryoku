package vault

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestExactValuesScrubberProtectsShortAndEncodedExportsConcurrently(t *testing.T) {
	values := []string{"q!", "exported/a+b=<value>&\"long"}
	grants := make(map[string]string, len(values))
	for _, value := range values {
		grants[value] = "EXPORTED_KEY"
	}
	scrub := ExactValuesScrubber(grants)
	var forms []string
	for _, value := range values {
		quoted, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		forms = append(forms, value,
			base64.StdEncoding.EncodeToString([]byte(value)), base64.RawStdEncoding.EncodeToString([]byte(value)),
			base64.URLEncoding.EncodeToString([]byte(value)), base64.RawURLEncoding.EncodeToString([]byte(value)),
			url.QueryEscape(value), url.PathEscape(value), string(quoted[1:len(quoted)-1]))
	}
	// Each caller shares one compiled scrubber, as parallel runtime callbacks do.
	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 10 {
				for _, form := range forms {
					output := scrub("VISIBLE-PREFIX " + form + " VISIBLE-SUFFIX")
					if strings.Contains(output, form) {
						t.Error("a short or encoded exported credential survived redaction")
					}
					if !strings.HasPrefix(output, "VISIBLE-PREFIX ") || !strings.HasSuffix(output, " VISIBLE-SUFFIX") {
						t.Error("redaction removed unrelated output")
					}
				}
			}
		}()
	}
	workers.Wait()
}
