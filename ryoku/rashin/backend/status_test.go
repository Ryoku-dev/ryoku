package main

import (
	"fmt"
	"net/http"
	"testing"
)

func TestNeedleReady(t *testing.T) {
	quickTestEnv(t)
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"routable":true,"harnesses":[]}`)
	})
	if !needleReady(HermesInfo{Configured: true}, defaultConfig()) {
		t.Fatal("configured hermes should be ready")
	}
	if !needleReady(HermesInfo{}, defaultConfig()) {
		t.Fatal("a routable Prowl gateway should make the needle ready")
	}
}
