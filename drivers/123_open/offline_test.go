package _123Open

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/alist-org/alist/v3/internal/model"
	pan123 "github.com/okatu-loli/go-123pan"
)

func TestOfflineDownload(t *testing.T) {
	var got map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/offline/download", func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeData(w, map[string]any{"taskID": 77})
	})
	d, _ := newTestDriver(t, mux)

	taskID, err := d.OfflineDownload(context.Background(), "https://example.com/a.iso", dirObj("300"), "a.iso")
	if err != nil {
		t.Fatalf("OfflineDownload: %v", err)
	}
	if taskID != 77 {
		t.Errorf("taskID = %d, want 77", taskID)
	}
	if got["url"] != "https://example.com/a.iso" || got["fileName"] != "a.iso" {
		t.Errorf("unexpected request body: %v", got)
	}
	if got["dirID"] != float64(300) {
		t.Errorf("dirID = %v, want 300", got["dirID"])
	}
}

// TestOfflineDownloadRejectsRoot guards against tasks silently landing in the
// platform's own "来自:离线下载" folder instead of the requested directory.
func TestOfflineDownloadRejectsRoot(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/offline/download", func(w http.ResponseWriter, r *http.Request) {
		t.Error("no task must be submitted for the root directory")
	})
	d, _ := newTestDriver(t, mux)

	_, err := d.OfflineDownload(context.Background(), "https://example.com/a.iso", dirObj("0"), "")
	if !errors.Is(err, errOfflineRootDir) {
		t.Fatalf("error = %v, want errOfflineRootDir", err)
	}
}

func TestOfflineProcess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/offline/download/process", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("taskID"); got != "77" {
			t.Errorf("taskID = %q, want 77", got)
		}
		writeData(w, map[string]any{"process": 42.5, "status": int(pan123.OfflineRetrying)})
	})
	d, _ := newTestDriver(t, mux)

	res, err := d.OfflineProcess(context.Background(), 77)
	if err != nil {
		t.Fatalf("OfflineProcess: %v", err)
	}
	if res.Process != 42.5 || res.Status != pan123.OfflineRetrying {
		t.Errorf("process = %+v, want 42.5 while retrying", res)
	}
}

func TestOfflineDownloadRejectsAMalformedDirID(t *testing.T) {
	d, _ := newTestDriver(t, http.NewServeMux())

	_, err := d.OfflineDownload(context.Background(), "https://example.com/a.iso",
		&model.Object{ID: "not-a-number", IsFolder: true}, "")
	if err == nil {
		t.Fatal("expected an error for a malformed directory id")
	}
}
