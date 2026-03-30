package exiftool

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestExiftoolVersion(t *testing.T) {
	ctx := context.Background()
	workDir := os.DirFS("testdata")

	out, err := Run(ctx, workDir, "-ver")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	ver := string(bytes.TrimSpace(out))
	t.Logf("exiftool version: %s", ver)
	if ver == "" {
		t.Fatal("empty version output")
	}
}

func TestExiftoolJSON(t *testing.T) {
	ctx := context.Background()
	workDir := os.DirFS("testdata")

	out, err := Run(ctx, workDir, "-json", "/work/test.jpg")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("no output from exiftool")
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, string(out))
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 image result, got %d", len(result))
	}

	fields := result[0]
	t.Logf("Got %d fields from test.jpg", len(fields))

	if v, ok := fields["ExifToolVersion"]; !ok {
		t.Error("missing ExifToolVersion field")
	} else {
		t.Logf("ExifToolVersion: %v", v)
	}

	if v, ok := fields["Make"]; !ok {
		t.Error("missing Make field")
	} else if v != "samsung" {
		t.Errorf("expected Make=samsung, got %v", v)
	}

	if v, ok := fields["Model"]; !ok {
		t.Error("missing Model field")
	} else if v != "SM-G930P" {
		t.Errorf("expected Model=SM-G930P, got %v", v)
	}
}

func TestExiftoolMultipleArgs(t *testing.T) {
	ctx := context.Background()
	workDir := os.DirFS("testdata")

	out, err := Run(ctx, workDir, "-json", "-Make", "-Model", "/work/test.jpg")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	fields := result[0]
	if len(fields) < 3 {
		t.Errorf("expected at least 3 fields (SourceFile, Make, Model), got %d", len(fields))
	}
	t.Logf("fields: %v", fields)
}
