package _123Open

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/stream"
	"github.com/alist-org/alist/v3/pkg/utils"
	pan123 "github.com/okatu-loli/go-123pan"
)

// newTestDriver wires a driver against a stand-in API server. The SDK fetches
// an access_token on its first call, so that route is always served.
func newTestDriver(t *testing.T, mux *http.ServeMux) (*Open123, *httptest.Server) {
	t.Helper()
	mux.HandleFunc("/api/v1/access_token", func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{
			"accessToken": "test-token",
			"expiredAt":   time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	d := &Open123{}
	d.AuthMode = AuthClientCredentials
	d.ClientID = "test-id"
	d.ClientSecret = "test-secret"
	d.RootFolderID = "0"
	d.UploadThread = 2
	d.ValidDuration = 30
	d.client = pan123.New("test-id", "test-secret",
		pan123.WithBaseURL(srv.URL),
		pan123.WithoutRateLimit(),
	)
	return d, srv
}

// writeData wraps a payload in the platform's standard success envelope.
func writeData(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    0,
		"message": "ok",
		"data":    data,
	})
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

func dirObj(id string) model.Obj {
	return &model.Object{ID: id, Name: "dir", IsFolder: true}
}

func TestListPagination(t *testing.T) {
	var gotQueries []url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/file/list", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotQueries = append(gotQueries, q)
		if q.Get("lastFileId") == "" {
			writeData(w, map[string]any{
				"lastFileId": 21,
				"fileList": []map[string]any{
					{
						"fileId": 11, "filename": "sub", "type": 1, "size": 0,
						"createAt": "2025-01-01 10:00:00", "updateAt": "2025-01-02 11:00:00",
					},
					{
						"fileId": 12, "filename": "a.txt", "type": 0, "size": 1024,
						"etag": "5d41402abc4b2a76b9719d911017c592", "trashed": 0,
						"createAt": "2025-01-01 10:00:00", "updateAt": "2025-01-03 12:30:45",
					},
					{
						"fileId": 13, "filename": "deleted.txt", "type": 0, "size": 7,
						"trashed": 1,
					},
				},
			})
			return
		}
		writeData(w, map[string]any{
			"lastFileId": -1,
			"fileList": []map[string]any{
				{"fileId": 21, "filename": "b.txt", "type": 0, "size": 42, "etag": "abc"},
			},
		})
	})
	d, _ := newTestDriver(t, mux)

	objs, err := d.List(context.Background(), dirObj("100"), model.ListArgs{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(gotQueries) != 2 {
		t.Fatalf("expected 2 list requests, got %d", len(gotQueries))
	}
	if got := gotQueries[0].Get("parentFileId"); got != "100" {
		t.Errorf("first page parentFileId = %q, want 100", got)
	}
	if got := gotQueries[0].Get("limit"); got != "100" {
		t.Errorf("first page limit = %q, want 100", got)
	}
	if got := gotQueries[1].Get("lastFileId"); got != "21" {
		t.Errorf("second page lastFileId = %q, want 21", got)
	}

	if len(objs) != 3 {
		t.Fatalf("expected 3 objects (trashed filtered out), got %d", len(objs))
	}
	if !objs[0].IsDir() || objs[0].GetID() != "11" || objs[0].GetName() != "sub" {
		t.Errorf("unexpected directory mapping: %+v", objs[0])
	}
	file := objs[1]
	if file.IsDir() {
		t.Error("a.txt should not be a directory")
	}
	if file.GetID() != "12" || file.GetSize() != 1024 {
		t.Errorf("unexpected file mapping: id=%s size=%d", file.GetID(), file.GetSize())
	}
	if got := file.GetHash().GetHash(utils.MD5); got != "5d41402abc4b2a76b9719d911017c592" {
		t.Errorf("md5 = %q, want the etag", got)
	}
	want := time.Date(2025, 1, 3, 12, 30, 45, 0, cst)
	if !file.ModTime().Equal(want) {
		t.Errorf("modified = %v, want %v", file.ModTime(), want)
	}
	if objs[2].GetID() != "21" {
		t.Errorf("second page object id = %q, want 21", objs[2].GetID())
	}
}

func TestLinkDownloadInfo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/file/download_info", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("fileId"); got != "12" {
			t.Errorf("fileId = %q, want 12", got)
		}
		writeData(w, map[string]any{"downloadUrl": "https://download.example.com/a.txt?x=1"})
	})
	mux.HandleFunc("/api/v1/direct-link/url", func(w http.ResponseWriter, r *http.Request) {
		t.Error("direct link must not be used when UseDirectLink is off")
	})
	d, _ := newTestDriver(t, mux)

	link, err := d.Link(context.Background(), &model.Object{ID: "12", Name: "a.txt"}, model.LinkArgs{})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if link.URL != "https://download.example.com/a.txt?x=1" {
		t.Errorf("url = %q, want the raw download url", link.URL)
	}
}

func TestLinkDirectLinkSigned(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/direct-link/url", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("fileID"); got != "12" {
			t.Errorf("fileID = %q, want 12", got)
		}
		writeData(w, map[string]any{"url": "https://direct.example.com/files/a.txt"})
	})
	d, _ := newTestDriver(t, mux)
	d.UseDirectLink = true
	d.PrivateKey = "secret-key"
	d.UID = 4242

	link, err := d.Link(context.Background(), &model.Object{ID: "12", Name: "a.txt"}, model.LinkArgs{})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	parsed, err := url.Parse(link.URL)
	if err != nil {
		t.Fatalf("parse signed url: %v", err)
	}
	if parsed.Host != "direct.example.com" || parsed.Path != "/files/a.txt" {
		t.Fatalf("signing changed the target: %s", link.URL)
	}
	authKey := parsed.Query().Get("auth_key")
	if authKey == "" {
		t.Fatal("auth_key is missing from the signed url")
	}
	parts := strings.Split(authKey, "-")
	if len(parts) != 4 {
		t.Fatalf("auth_key = %q, want ts-rand-uid-md5", authKey)
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		t.Fatalf("auth_key timestamp: %v", err)
	}
	if ts < time.Now().Unix() {
		t.Errorf("auth_key expires at %d, which is already in the past", ts)
	}
	if parts[2] != "4242" {
		t.Errorf("auth_key uid = %q, want 4242", parts[2])
	}
	want := md5.Sum([]byte(fmt.Sprintf("%s-%s-%s-%s-%s", parsed.Path, parts[0], parts[1], parts[2], "secret-key")))
	if parts[3] != hex.EncodeToString(want[:]) {
		t.Errorf("auth_key signature = %q, want %x", parts[3], want)
	}

	// without a private key the url is handed back untouched
	d.PrivateKey = ""
	link, err = d.Link(context.Background(), &model.Object{ID: "12", Name: "a.txt"}, model.LinkArgs{})
	if err != nil {
		t.Fatalf("Link without signing: %v", err)
	}
	if link.URL != "https://direct.example.com/files/a.txt" {
		t.Errorf("unsigned url = %q", link.URL)
	}
}

func TestLinkOfDirIsRejected(t *testing.T) {
	d, _ := newTestDriver(t, http.NewServeMux())
	if _, err := d.Link(context.Background(), dirObj("1"), model.LinkArgs{}); err == nil {
		t.Fatal("expected an error for a directory")
	}
}

func TestMakeDir(t *testing.T) {
	var got map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/upload/v1/file/mkdir", func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeData(w, map[string]any{"dirID": 777})
	})
	d, _ := newTestDriver(t, mux)

	obj, err := d.MakeDir(context.Background(), dirObj("100"), "new folder")
	if err != nil {
		t.Fatalf("MakeDir: %v", err)
	}
	if got["name"] != "new folder" {
		t.Errorf("name = %v, want \"new folder\"", got["name"])
	}
	if got["parentID"] != float64(100) {
		t.Errorf("parentID = %v, want 100", got["parentID"])
	}
	if obj.GetID() != "777" || obj.GetName() != "new folder" || !obj.IsDir() {
		t.Errorf("unexpected created object: %+v", obj)
	}
}

func TestRemove(t *testing.T) {
	var got map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/file/trash", func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeData(w, nil)
	})
	d, _ := newTestDriver(t, mux)

	if err := d.Remove(context.Background(), &model.Object{ID: "12", Name: "a.txt"}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	ids, ok := got["fileIDs"].([]any)
	if !ok || len(ids) != 1 || ids[0] != float64(12) {
		t.Fatalf("fileIDs = %v, want [12]", got["fileIDs"])
	}
}

func TestRemoveReportsAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/file/trash", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 5066, "message": "file not found"})
	})
	d, _ := newTestDriver(t, mux)

	err := d.Remove(context.Background(), &model.Object{ID: "12", Name: "a.txt"})
	if err == nil {
		t.Fatal("expected the api error to be surfaced")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("error = %v, want it to mention the api message", err)
	}
}

func TestRenameAndMove(t *testing.T) {
	var renameBody, moveBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/file/name", func(w http.ResponseWriter, r *http.Request) {
		renameBody = decodeBody(t, r)
		writeData(w, nil)
	})
	mux.HandleFunc("/api/v1/file/move", func(w http.ResponseWriter, r *http.Request) {
		moveBody = decodeBody(t, r)
		writeData(w, nil)
	})
	d, _ := newTestDriver(t, mux)

	src := &model.Object{ID: "12", Name: "a.txt", Size: 1024}
	renamed, err := d.Rename(context.Background(), src, "b.txt")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renameBody["fileId"] != float64(12) || renameBody["fileName"] != "b.txt" {
		t.Errorf("unexpected rename body: %v", renameBody)
	}
	if renamed.GetName() != "b.txt" || renamed.GetID() != "12" {
		t.Errorf("unexpected renamed object: %+v", renamed)
	}

	moved, err := d.Move(context.Background(), src, dirObj("200"))
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if moveBody["toParentFileID"] != float64(200) {
		t.Errorf("toParentFileID = %v, want 200", moveBody["toParentFileID"])
	}
	if moved.GetID() != "12" || moved.GetSize() != 1024 {
		t.Errorf("unexpected moved object: %+v", moved)
	}
}

func TestCopyPollsUntilDone(t *testing.T) {
	old := copyPollInterval
	copyPollInterval = time.Millisecond
	t.Cleanup(func() { copyPollInterval = old })

	var mu sync.Mutex
	polls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/file/async/copy", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		if body["targetDirId"] != float64(200) {
			t.Errorf("targetDirId = %v, want 200", body["targetDirId"])
		}
		writeData(w, map[string]any{"taskId": 9})
	})
	mux.HandleFunc("/api/v1/file/async/copy/process", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		polls++
		n := polls
		mu.Unlock()
		status := int(pan123.CopyTaskRunning)
		if n >= 3 {
			status = int(pan123.CopyTaskDone)
		}
		writeData(w, map[string]any{"taskId": 9, "status": status})
	})
	d, _ := newTestDriver(t, mux)

	err := d.Copy(context.Background(), &model.Object{ID: "12", Name: "a.txt"}, dirObj("200"))
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if polls < 3 {
		t.Errorf("copy polled %d times, want at least 3", polls)
	}
}

func TestCopyReportsFailedTask(t *testing.T) {
	old := copyPollInterval
	copyPollInterval = time.Millisecond
	t.Cleanup(func() { copyPollInterval = old })

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/file/async/copy", func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{"taskId": 9})
	})
	mux.HandleFunc("/api/v1/file/async/copy/process", func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{"taskId": 9, "status": int(pan123.CopyTaskFailed)})
	})
	d, _ := newTestDriver(t, mux)

	if err := d.Copy(context.Background(), &model.Object{ID: "12"}, dirObj("200")); err == nil {
		t.Fatal("expected a failed copy task to be reported")
	}
}

func TestPutInstantUpload(t *testing.T) {
	const content = "hello"
	sum := md5.Sum([]byte(content))
	etag := hex.EncodeToString(sum[:])

	var created map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/upload/v2/file/create", func(w http.ResponseWriter, r *http.Request) {
		created = decodeBody(t, r)
		writeData(w, map[string]any{"fileID": 555, "reuse": true})
	})
	mux.HandleFunc("/upload/v2/file/slice", func(w http.ResponseWriter, r *http.Request) {
		t.Error("no slice must be uploaded when the server reports an instant upload")
	})
	d, _ := newTestDriver(t, mux)

	var progress float64
	fs := &stream.FileStream{
		Obj: &model.Object{
			Name:     "a.txt",
			Size:     int64(len(content)),
			Modified: time.Now(),
			HashInfo: utils.NewHashInfo(utils.MD5, etag),
		},
		Reader: strings.NewReader(content),
	}
	obj, err := d.Put(context.Background(), dirObj("100"), fs, func(p float64) { progress = p })
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if created["etag"] != etag {
		t.Errorf("etag = %v, want %s", created["etag"], etag)
	}
	if created["parentFileID"] != float64(100) || created["filename"] != "a.txt" {
		t.Errorf("unexpected create body: %v", created)
	}
	if created["size"] != float64(len(content)) {
		t.Errorf("size = %v, want %d", created["size"], len(content))
	}
	if obj.GetID() != "555" || obj.GetName() != "a.txt" {
		t.Errorf("unexpected uploaded object: %+v", obj)
	}
	if obj.GetHash().GetHash(utils.MD5) != etag {
		t.Errorf("uploaded object lost its md5")
	}
	if progress != 100 {
		t.Errorf("progress = %v, want 100", progress)
	}
}

func TestPutUploadsSlices(t *testing.T) {
	old := completePollInterval
	completePollInterval = time.Millisecond
	t.Cleanup(func() { completePollInterval = old })

	content := strings.Repeat("0123456789", 25) // 250 bytes, 3 slices of 100
	const sliceSize = 100

	var (
		mu        sync.Mutex
		slices    = map[int64][]byte{}
		md5s      = map[int64]string{}
		completes int
	)
	mux := http.NewServeMux()
	var serverURL string
	mux.HandleFunc("/upload/v2/file/create", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		if body["etag"] == "" {
			t.Error("create was called without an etag")
		}
		writeData(w, map[string]any{
			"preuploadID": "pre-1",
			"reuse":       false,
			"sliceSize":   sliceSize,
			"servers":     []string{serverURL},
		})
	})
	mux.HandleFunc("/upload/v2/file/slice", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			return
		}
		if r.FormValue("preuploadID") != "pre-1" {
			t.Errorf("preuploadID = %q", r.FormValue("preuploadID"))
		}
		no, err := strconv.ParseInt(r.FormValue("sliceNo"), 10, 64)
		if err != nil {
			t.Errorf("sliceNo: %v", err)
			return
		}
		part, _, err := r.FormFile("slice")
		if err != nil {
			t.Errorf("slice part: %v", err)
			return
		}
		defer part.Close()
		data, _ := io.ReadAll(part)
		mu.Lock()
		slices[no] = data
		md5s[no] = r.FormValue("sliceMD5")
		mu.Unlock()
		writeData(w, nil)
	})
	mux.HandleFunc("/upload/v2/file/upload_complete", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		completes++
		n := completes
		mu.Unlock()
		if n < 2 {
			// the server is still merging
			writeData(w, map[string]any{"completed": false})
			return
		}
		writeData(w, map[string]any{"completed": true, "fileID": 999})
	})
	d, srv := newTestDriver(t, mux)
	serverURL = srv.URL

	// an *os.File reader lets the stream serve slices without a temp dir
	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	var maxProgress float64
	fs := &stream.FileStream{
		Obj:    &model.Object{Name: "a.txt", Size: int64(len(content)), Modified: time.Now()},
		Reader: f,
	}
	obj, err := d.Put(context.Background(), dirObj("100"), fs, func(p float64) {
		if p > maxProgress {
			maxProgress = p
		}
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if obj.GetID() != "999" {
		t.Errorf("uploaded object id = %q, want 999", obj.GetID())
	}
	if len(slices) != 3 {
		t.Fatalf("uploaded %d slices, want 3", len(slices))
	}
	var reassembled []byte
	for no := int64(1); no <= 3; no++ {
		data, ok := slices[no]
		if !ok {
			t.Fatalf("slice %d is missing", no)
		}
		sum := md5.Sum(data)
		if md5s[no] != hex.EncodeToString(sum[:]) {
			t.Errorf("slice %d md5 = %q, want %x", no, md5s[no], sum)
		}
		reassembled = append(reassembled, data...)
	}
	if string(reassembled) != content {
		t.Error("the reassembled slices do not match the source content")
	}
	if completes < 2 {
		t.Errorf("upload_complete was called %d times, want it to be polled", completes)
	}
	if maxProgress != 100 {
		t.Errorf("progress reached %v, want 100", maxProgress)
	}
}
