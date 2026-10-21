package _123Open

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/stream"
)

// writeAPIError answers with the platform's business error envelope.
func writeAPIError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
		"data":    nil,
	})
}

// completeMux serves a slice upload whose completion answers with business
// errors: the first errCount calls, or every call when alwaysFail is set.
func completeMux(errCount int, alwaysFail bool) (*http.ServeMux, *int, *string) {
	calls := 0
	serverURL := ""
	mux := http.NewServeMux()
	mux.HandleFunc("/upload/v2/file/create", func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{
			"preuploadID": "pre-1",
			"reuse":       false,
			"sliceSize":   8,
			"servers":     []string{serverURL},
		})
	})
	mux.HandleFunc("/upload/v2/file/slice", func(w http.ResponseWriter, r *http.Request) {
		writeData(w, nil)
	})
	mux.HandleFunc("/upload/v2/file/upload_complete", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if alwaysFail || calls <= errCount {
			// observed live: the platform reports an undocumented business
			// error while it is still merging the slices
			writeAPIError(w, 20103, "文件正在处理中")
			return
		}
		writeData(w, map[string]any{"completed": true, "fileID": 987})
	})
	return mux, &calls, &serverURL
}

// putTestStream backs the stream with an *os.File so slices can be served
// without a configured temp dir.
func putTestStream(t *testing.T, name, content string) model.FileStreamer {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return &stream.FileStream{
		Obj:    &model.Object{Name: name, Size: int64(len(content)), Modified: time.Now()},
		Reader: f,
	}
}

func shrinkCompletePolling(t *testing.T, maxPolls int) {
	t.Helper()
	oldInterval, oldMax := completePollInterval, completeMaxPolls
	completePollInterval, completeMaxPolls = time.Millisecond, maxPolls
	t.Cleanup(func() { completePollInterval, completeMaxPolls = oldInterval, oldMax })
}

// The platform merges slices asynchronously and, while it is still working,
// upload_complete answers with a business error (observed live: code 20103,
// "文件正在..."), even though the file does land. The poll must read that as
// "not finished yet" instead of failing the upload.
func TestPutKeepsPollingWhileServerMerges(t *testing.T) {
	mux, calls, serverURL := completeMux(2, false)
	d, srv := newTestDriver(t, mux)
	*serverURL = srv.URL
	shrinkCompletePolling(t, 20)

	obj, err := d.Put(context.Background(), &model.Object{ID: "0", IsFolder: true},
		putTestStream(t, "merge.bin", "0123456789abcdef"), func(float64) {})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if *calls != 3 {
		t.Fatalf("upload_complete called %d times, want 3 (two 'still merging' answers, then success)", *calls)
	}
	if obj.GetID() != "987" {
		t.Fatalf("file id = %s, want 987", obj.GetID())
	}
}

// A completion that never finishes must give up with a diagnosable error
// rather than poll forever.
func TestPutGivesUpWhenCompletionNeverFinishes(t *testing.T) {
	mux, calls, serverURL := completeMux(0, true)
	d, srv := newTestDriver(t, mux)
	*serverURL = srv.URL
	shrinkCompletePolling(t, 3)

	_, err := d.Put(context.Background(), &model.Object{ID: "0", IsFolder: true},
		putTestStream(t, "stuck.bin", "0123456789abcdef"), func(float64) {})
	if err == nil {
		t.Fatal("expected an error when the server never reports completion")
	}
	if !strings.Contains(err.Error(), "stuck.bin") {
		t.Errorf("error = %q, want it to name the file", err)
	}
	if *calls != 3 {
		t.Fatalf("upload_complete called %d times, want it bounded at 3", *calls)
	}
}
