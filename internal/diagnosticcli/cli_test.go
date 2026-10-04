package diagnosticcli

import (
	"io"
	"testing"
)

func TestDiagnoseArgs(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		host  string
		valid bool
	}{
		{[]string{"diagnose"}, "", true},
		{[]string{"diagnose", "--host", "tmcars.info"}, "tmcars.info", true},
		{[]string{"diagnose", "--host=8.8.8.8"}, "8.8.8.8", true},
		{[]string{"diagnose", "--host"}, "", false},
		{[]string{"diagnose", "--host="}, "", false},
		{[]string{"diagnose", "--host", "https://tmcars.info"}, "", false},
		{[]string{"diagnose", "extra"}, "", false},
		{[]string{"unknown"}, "", false},
	} {
		host, err := parseDiagnoseArgs(tc.args, io.Discard)
		if (err == nil) != tc.valid || (tc.valid && host != tc.host) {
			t.Fatalf("%v: %q, %v", tc.args, host, err)
		}
	}
}

func TestAIOptions(t *testing.T) {
	t.Setenv("OLLAMA_MODEL", "env-model")
	t.Setenv("OLLAMA_HOST", "http://localhost:11434")
	options, err := parseOptions([]string{"diagnose", "--ai"}, io.Discard)
	if err != nil || !options.ai || options.model != "env-model" {
		t.Fatalf("%+v %v", options, err)
	}
	options, err = parseOptions([]string{"diagnose", "--model", "flag-model", "--ollama-url", "http://localhost:9999"}, io.Discard)
	if err != nil || options.ai || options.model != "flag-model" || options.ollamaURL != "http://localhost:9999" {
		t.Fatalf("%+v %v", options, err)
	}
}
