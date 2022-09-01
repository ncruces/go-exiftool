package exiftool

import (
	"bytes"
	"strconv"
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
