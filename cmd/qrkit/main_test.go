package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "x.svg")
	err := run([]string{"-o", out, "-shape", "rounded", "-fg", "#123", "-bg", "transparent", "hello"}, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(out); err != nil || st.Size() == 0 {
		t.Fatal("no output written")
	}
	var buf bytes.Buffer
	if err := run([]string{"-terminal", "-"}, strings.NewReader("from stdin\n"), &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\x1b[40m") {
		t.Error("terminal output missing")
	}
	for _, bad := range [][]string{{}, {"-o", out, "-shape", "nope", "x"}, {"-o", out, "-fg", "zzz", "x"}, {"x"}} {
		if err := run(bad, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("run(%v) should fail", bad)
		}
	}
}

func TestParseColor(t *testing.T) {
	for _, ok := range []string{"#fff", "#FFFFFF", "#11223344", "transparent", "black"} {
		if _, err := parseColor(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	if _, err := parseColor("#12"); err == nil {
		t.Error("expected error")
	}
}
