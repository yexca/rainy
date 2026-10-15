package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestBackupCLI(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"help"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "backup create") || !strings.Contains(out.String(), "backup verify") {
		t.Fatal("backup commands missing from help")
	}
	if err := run([]string{"backup", "restore", "example"}, &out, &out); err == nil {
		t.Fatal("unknown operation accepted")
	}
	if err := run([]string{"backup", "create"}, &out, &out); err == nil {
		t.Fatal("missing name accepted")
	}
}
