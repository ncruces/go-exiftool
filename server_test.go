package exiftool

import (
	"bytes"
	stderrors "errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func TestServer(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	// ask for version number
	out, err := e.Command("-ver")
	if err != nil {
		t.Error(err)
	} else if ver, err := strconv.ParseFloat(string(bytes.TrimSpace(out)), 64); err != nil {
		t.Error(err)
	} else {
		t.Log(ver)
	}

	// shutdown the server
	err = e.Shutdown()
	if err != nil {
		t.Error(err)
	}

	// shutdown should not be called twice
	err = e.Shutdown()
	if err == nil {
		t.Error("repeated shutdown")
	}

	// commands should fail now
	_, err = e.Command("-ver")
	if err == nil {
		t.Error("command after shutdown")
	}

	// close should be fine at any time
	err = e.Close()
	if err != nil {
		t.Error(err)
	}
}

func TestServerReadTags(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("-Artist", "-Copyright", "-Make", "-Model", "testdata/sample.jpg")
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if got, want := string(m["Artist"]), "Test Artist"; got != want {
		t.Errorf("Artist: got %q, want %q", got, want)
	}
	if got, want := string(m["Copyright"]), "Test Copyright 2024"; got != want {
		t.Errorf("Copyright: got %q, want %q", got, want)
	}
	if got, want := string(m["Make"]), "TestMaker"; got != want {
		t.Errorf("Make: got %q, want %q", got, want)
	}
	if got, want := string(m["Camera Model Name"]), "TestModel"; got != want {
		t.Errorf("Camera Model Name: got %q, want %q", got, want)
	}
}

func TestServerReadAllTags(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("testdata/sample.jpg")
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	expectedTags := []string{"Artist", "Copyright", "Make", "Camera Model Name", "Software", "File Name"}
	for _, tag := range expectedTags {
		if _, ok := m[tag]; !ok {
			t.Errorf("expected tag %q not found", tag)
		}
	}
	t.Logf("Found %d tags", len(m))
}

func TestServerGPSTags(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("-GPSLatitude", "-GPSLongitude", "-GPSAltitude", "testdata/sample_gps.jpg")
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if _, ok := m["GPS Latitude"]; !ok {
		t.Error("expected GPS Latitude tag")
	}
	if _, ok := m["GPS Longitude"]; !ok {
		t.Error("expected GPS Longitude tag")
	}
	if _, ok := m["GPS Altitude"]; !ok {
		t.Error("expected GPS Altitude tag")
	}
}

func TestServerMultipleCommands(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Execute multiple sequential commands
	for i := 0; i < 5; i++ {
		out, err := e.Command("-ver")
		if err != nil {
			t.Fatalf("command %d: %v", i, err)
		}
		if len(bytes.TrimSpace(out)) == 0 {
			t.Errorf("command %d: empty output", i)
		}
	}
}

func TestServerMultipleFiles(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("-FileName", "-Artist", "testdata/sample.jpg", "testdata/sample_gps.jpg")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(out, []byte("sample.jpg")) {
		t.Error("expected sample.jpg in output")
	}
	if !bytes.Contains(out, []byte("sample_gps.jpg")) {
		t.Error("expected sample_gps.jpg in output")
	}
}

func TestServerNonexistentFile(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	_, err = e.Command("testdata/nonexistent.jpg")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	t.Logf("Error (expected): %v", err)
}

func TestServerShortFormat(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("-s", "-Artist", "testdata/sample.jpg")
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if got, want := string(m["Artist"]), "Test Artist"; got != want {
		t.Errorf("Artist: got %q, want %q", got, want)
	}
}

func TestServerWriteTag(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.jpg")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "write_test.jpg")
	if err := os.WriteFile(tmpFile, src, 0644); err != nil {
		t.Fatal(err)
	}

	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Write a new tag
	_, err = e.Command("-Artist=Server Artist", "-overwrite_original", tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the tag was written
	out, err := e.Command("-Artist", tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if got, want := string(m["Artist"]), "Server Artist"; got != want {
		t.Errorf("Artist: got %q, want %q", got, want)
	}
}

func TestServerDeleteTag(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.jpg")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "delete_test.jpg")
	if err := os.WriteFile(tmpFile, src, 0644); err != nil {
		t.Fatal(err)
	}

	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Delete the Artist tag
	_, err = e.Command("-Artist=", "-overwrite_original", tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the tag was deleted
	out, err := e.Command("-Artist", tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if _, ok := m["Artist"]; ok {
		t.Error("Artist tag should have been deleted")
	}
}

func TestServerCopyTags(t *testing.T) {
	src, err := os.ReadFile("testdata/sample_noexif.jpg")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	dstFile := filepath.Join(tmpDir, "copy_dst.jpg")
	if err := os.WriteFile(dstFile, src, 0644); err != nil {
		t.Fatal(err)
	}

	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Copy tags
	_, err = e.Command("-TagsFromFile", "testdata/sample.jpg", "-Artist", "-Copyright", "-overwrite_original", dstFile)
	if err != nil {
		t.Fatal(err)
	}

	// Verify tags were copied
	out, err := e.Command("-Artist", "-Copyright", dstFile)
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if got, want := string(m["Artist"]), "Test Artist"; got != want {
		t.Errorf("Artist: got %q, want %q", got, want)
	}
	if got, want := string(m["Copyright"]), "Test Copyright 2024"; got != want {
		t.Errorf("Copyright: got %q, want %q", got, want)
	}
}

func TestServerJSONOutput(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("-json", "-Artist", "-Copyright", "testdata/sample.jpg")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(out, []byte(`"Artist"`)) {
		t.Error("expected JSON with Artist key")
	}
	if !bytes.Contains(out, []byte(`"Test Artist"`)) {
		t.Error("expected JSON with Test Artist value")
	}
}

func TestServerClose(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	// Force close
	err = e.Close()
	if err != nil {
		t.Error(err)
	}

	// Double close should be fine
	err = e.Close()
	if err != nil {
		t.Error(err)
	}
}

func TestServerConcurrentCommands(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			out, err := e.Command("-ver")
			if err != nil {
				errs <- err
				return
			}
			if len(bytes.TrimSpace(out)) == 0 {
				errs <- stderrors.New("empty output")
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestServerConcurrentReads(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	var wg sync.WaitGroup
	errors := make(chan error, 5)

	files := []string{
		"testdata/sample.jpg",
		"testdata/sample_gps.jpg",
		"testdata/sample_noexif.jpg",
		"testdata/sample.png",
		"testdata/sample.tiff",
	}

	for _, f := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			_, err := e.Command("-FileType", file)
			if err != nil {
				errors <- err
			}
		}(f)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestServerCommonArgs(t *testing.T) {
	// Create server with common args
	e, err := NewServer("-fast")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("testdata/sample.jpg")
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if len(m) == 0 {
		t.Error("expected some tags")
	}
}

func TestServerPNG(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("-FileType", "testdata/sample.png")
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if got, want := string(m["File Type"]), "PNG"; got != want {
		t.Errorf("File Type: got %q, want %q", got, want)
	}
}

func TestServerTIFF(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	out, err := e.Command("-Artist", "testdata/sample.tiff")
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string][]byte)
	if err := Unmarshal(out, m); err != nil {
		t.Fatal(err)
	}

	if got, want := string(m["Artist"]), "TIFF Artist"; got != want {
		t.Errorf("Artist: got %q, want %q", got, want)
	}
}
