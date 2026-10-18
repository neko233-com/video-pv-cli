package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestQAEventCounterDistinguishesSubtestsSkipsAndPackages(t *testing.T) {
	raw := `{"Action":"pass","Test":"TestOne"}
{"Action":"pass","Test":"TestOne/sub"}
{"Action":"pass","Package":"root"}
{"Action":"skip","Test":"TestGPU"}
{"Action":"fail","Test":"TestBad"}
`
	pass, skip, fail, err := countGoTestEvents(strings.NewReader(raw))
	if err != nil || pass != 2 || skip != 1 || fail != 1 {
		t.Fatal(pass, skip, fail, err)
	}
	if _, _, _, err := countGoTestEvents(strings.NewReader("broken")); err == nil {
		t.Fatal("invalid test log accepted")
	}
}
func TestQARejectsWrongSourceAndOptions(t *testing.T) {
	var out bytes.Buffer
	for _, args := range [][]string{{"qa"}, {"qa", "windows", "--count", "0"}, {"qa", "windows", "--source", t.TempDir()}} {
		if realMain(args, &out, &out) != 2 {
			t.Fatal(args, out.String())
		}
	}
}
