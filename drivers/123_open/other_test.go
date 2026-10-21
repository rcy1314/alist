package _123Open

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	pan123 "github.com/okatu-loli/go-123pan"
)

// otherCase describes one Other method: the route it is expected to hit, the
// payload the stand-in server answers with and the assertions on both the
// request the SDK sent and the value handed back to the caller.
type otherCase struct {
	name     string
	method   string
	obj      model.Obj
	data     interface{}
	route    string
	response interface{}
	// wantQuery is checked against the query string of a GET request.
	wantQuery map[string]string
	// wantBody is checked against the JSON body of a POST/PUT request.
	wantBody map[string]interface{}
	// check inspects the decoded result of Other.
	check func(t *testing.T, got map[string]interface{})
}

func TestOtherDispatch(t *testing.T) {
	cases := []otherCase{
		{
			name:     "user_info",
			method:   "user_info",
			route:    "/api/v1/user/info",
			response: map[string]any{"uid": 4242, "nickname": "tester", "spaceUsed": 1024, "vip": true},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["uid"] != float64(4242) || got["nickname"] != "tester" {
					t.Errorf("user info = %v, want the full payload", got)
				}
				if got["spaceUsed"] != float64(1024) {
					t.Errorf("spaceUsed = %v, want 1024", got["spaceUsed"])
				}
			},
		},
		{
			name:     "detail falls back to the addressed object",
			method:   "detail",
			obj:      &model.Object{ID: "12", Name: "a.txt"},
			route:    "/api/v1/file/detail",
			response: map[string]any{"fileID": 12, "filename": "a.txt", "size": 7},
			wantQuery: map[string]string{
				"fileID": "12",
			},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["filename"] != "a.txt" {
					t.Errorf("detail = %v, want a.txt", got)
				}
			},
		},
		{
			name:   "recover_to decodes its arguments",
			method: "recover_to",
			data: map[string]any{
				"file_ids":       []int64{11, 12},
				"parent_file_id": 200,
			},
			route:    "/api/v1/file/recover/by_path",
			response: nil,
			wantBody: map[string]interface{}{
				"fileIDs":      []interface{}{float64(11), float64(12)},
				"parentFileID": float64(200),
			},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["success"] != true {
					t.Errorf("recover_to = %v, want success", got)
				}
			},
		},
		{
			name:   "share_create",
			method: "share_create",
			data: map[string]any{
				"share_name":   "holiday",
				"share_expire": 7,
				"file_ids":     []int64{11, 12},
				"share_pwd":    "1234",
			},
			route:    "/api/v1/share/create",
			response: map[string]any{"shareID": 900, "shareKey": "abcdef"},
			wantBody: map[string]interface{}{
				"shareName":   "holiday",
				"shareExpire": float64(7),
				"fileIDList":  "11,12",
				"sharePwd":    "1234",
			},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["shareID"] != float64(900) || got["shareKey"] != "abcdef" {
					t.Errorf("share_create = %v, want the sdk result", got)
				}
			},
		},
		{
			name:      "share_list paging arguments",
			method:    "share_list",
			data:      map[string]any{"limit": 50, "last_share_id": 33},
			route:     "/api/v1/share/list",
			response:  map[string]any{"lastShareId": -1, "shareList": []map[string]any{{"shareId": 900}}},
			wantQuery: map[string]string{"limit": "50", "lastShareId": "33"},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["lastShareId"] != float64(-1) {
					t.Errorf("share_list = %v, want lastShareId -1", got)
				}
			},
		},
		{
			name:      "direct_link_url",
			method:    "direct_link_url",
			data:      map[string]any{"file_id": 12},
			route:     "/api/v1/direct-link/url",
			response:  map[string]any{"url": "https://direct.example.com/a.txt"},
			wantQuery: map[string]string{"fileID": "12"},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["url"] != "https://direct.example.com/a.txt" {
					t.Errorf("direct_link_url = %v", got)
				}
			},
		},
		{
			name:     "direct_link_ip_blacklist",
			method:   "direct_link_ip_blacklist",
			route:    "/api/v1/developer/config/forbide-ip/list",
			response: map[string]any{"ipList": []string{"1.2.3.4"}, "status": 1},
			check: func(t *testing.T, got map[string]interface{}) {
				ips, ok := got["ip_list"].([]interface{})
				if !ok || len(ips) != 1 || ips[0] != "1.2.3.4" {
					t.Errorf("ip_list = %v, want [1.2.3.4]", got["ip_list"])
				}
				if got["status"] != float64(1) {
					t.Errorf("status = %v, want 1", got["status"])
				}
			},
		},
		{
			name:   "offline_download",
			method: "offline_download",
			data: map[string]any{
				"url":       "https://example.com/a.iso",
				"file_name": "a.iso",
				"dir_id":    300,
			},
			route:    "/api/v1/offline/download",
			response: map[string]any{"taskID": 77},
			wantBody: map[string]interface{}{
				"url":      "https://example.com/a.iso",
				"fileName": "a.iso",
				"dirID":    float64(300),
			},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["task_id"] != float64(77) {
					t.Errorf("offline_download = %v, want task_id 77", got)
				}
			},
		},
		{
			name:      "offline_process",
			method:    "offline_process",
			data:      map[string]any{"task_id": 77},
			route:     "/api/v1/offline/download/process",
			response:  map[string]any{"process": 42.5, "status": 0},
			wantQuery: map[string]string{"taskID": "77"},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["process"] != 42.5 {
					t.Errorf("process = %v, want 42.5", got["process"])
				}
			},
		},
		{
			name:     "transcode_resolutions",
			method:   "transcode_resolutions",
			data:     map[string]any{"file_id": 12},
			route:    "/api/v1/transcode/video/resolutions",
			response: map[string]any{"IsGetResolution": false, "Resolutions": "480p,720p", "VideoTime": 61},
			wantBody: map[string]interface{}{"fileId": float64(12)},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["Resolutions"] != "480p,720p" {
					t.Errorf("transcode_resolutions = %v", got)
				}
			},
		},
		{
			name:   "transcode_download_ts",
			method: "transcode_download_ts",
			data: map[string]any{
				"file_id":    12,
				"resolution": "1080P",
				"ts_name":    "000",
			},
			route:    "/api/v1/transcode/m3u8_ts/download",
			response: map[string]any{"downloadUrl": "https://ts.example.com/000.ts", "isFull": false},
			wantBody: map[string]interface{}{
				"fileId":     float64(12),
				"resolution": "1080P",
				"tsName":     "000",
				"type":       float64(2),
			},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["downloadUrl"] != "https://ts.example.com/000.ts" {
					t.Errorf("transcode_download_ts = %v", got)
				}
			},
		},
		{
			name:     "oss_mkdir",
			method:   "oss_mkdir",
			data:     map[string]any{"parent_id": "root-id", "name": "pictures"},
			route:    "/upload/v1/oss/file/mkdir",
			response: map[string]any{"list": []map[string]any{{"filename": "pictures", "dirID": "dir-1"}}},
			wantBody: map[string]interface{}{
				"parentID": "root-id",
				"name":     "pictures",
				"type":     float64(1),
			},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["dir_id"] != "dir-1" {
					t.Errorf("oss_mkdir = %v, want dir-1", got)
				}
			},
		},
		{
			name:     "oss_copy_from_disk",
			method:   "oss_copy_from_disk",
			data:     map[string]any{"file_ids": []int64{11}, "to_parent_file_id": "dir-1"},
			route:    "/api/v1/oss/source/copy",
			response: map[string]any{"taskID": "task-9"},
			wantBody: map[string]interface{}{
				"fileIDs":        []interface{}{"11"},
				"toParentFileID": "dir-1",
			},
			check: func(t *testing.T, got map[string]interface{}) {
				if got["task_id"] != "task-9" {
					t.Errorf("oss_copy_from_disk = %v, want task-9", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var (
				hits  int
				query map[string]string
				body  map[string]interface{}
			)
			mux := http.NewServeMux()
			mux.HandleFunc(tc.route, func(w http.ResponseWriter, r *http.Request) {
				hits++
				if r.Method == http.MethodGet {
					query = map[string]string{}
					for k, v := range r.URL.Query() {
						query[k] = v[0]
					}
				} else {
					_ = json.NewDecoder(r.Body).Decode(&body)
				}
				writeData(w, tc.response)
			})
			d, _ := newTestDriver(t, mux)

			res, err := d.Other(context.Background(), model.OtherArgs{
				Obj:    tc.obj,
				Method: tc.method,
				Data:   tc.data,
			})
			if err != nil {
				t.Fatalf("Other(%s): %v", tc.method, err)
			}
			if hits != 1 {
				t.Fatalf("route %s was called %d times, want 1", tc.route, hits)
			}
			for k, want := range tc.wantQuery {
				if query[k] != want {
					t.Errorf("query %s = %q, want %q", k, query[k], want)
				}
			}
			for k, want := range tc.wantBody {
				if !jsonEqual(body[k], want) {
					t.Errorf("body %s = %#v, want %#v", k, body[k], want)
				}
			}
			if tc.check != nil {
				tc.check(t, toMap(t, res))
			}
		})
	}
}

// TestOtherMethodIsNormalized proves the dispatch is case and space tolerant.
func TestOtherMethodIsNormalized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/info", func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{"uid": 1})
	})
	d, _ := newTestDriver(t, mux)

	if _, err := d.Other(context.Background(), model.OtherArgs{Method: "  User_Info "}); err != nil {
		t.Fatalf("Other: %v", err)
	}
}

func TestOtherUnknownMethod(t *testing.T) {
	d, _ := newTestDriver(t, http.NewServeMux())

	_, err := d.Other(context.Background(), model.OtherArgs{Method: "does_not_exist"})
	if !errors.Is(err, errs.NotSupport) {
		t.Fatalf("error = %v, want errs.NotSupport", err)
	}
	if _, err := d.Other(context.Background(), model.OtherArgs{Method: ""}); !errors.Is(err, errs.NotSupport) {
		t.Fatalf("empty method error = %v, want errs.NotSupport", err)
	}
}

// TestOtherRequiresArguments covers the guards that reject a request the API
// would only reject after a round trip.
func TestOtherRequiresArguments(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/oss/file/detail", func(w http.ResponseWriter, r *http.Request) {
		t.Error("the request must not reach the server without a file id")
	})
	d, _ := newTestDriver(t, mux)

	for _, tc := range []struct {
		method string
		data   interface{}
		want   string
	}{
		{"oss_detail", map[string]any{}, "file_id is required"},
		{"detail", nil, "file_id is required"},
		{"transcode_download_all", map[string]any{"file_id": 12}, "zip_name is required"},
		{"direct_link_ip_blacklist_switch", map[string]any{"status": 9}, "status must be"},
		{"share_update", map[string]any{}, "share_ids is required"},
	} {
		_, err := d.Other(context.Background(), model.OtherArgs{Method: tc.method, Data: tc.data})
		if err == nil {
			t.Errorf("%s: expected an error", tc.method)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want it to mention %q", tc.method, err, tc.want)
		}
		if !strings.HasPrefix(err.Error(), tc.method+" failed: ") {
			t.Errorf("%s: error = %v, want it to name the method", tc.method, err)
		}
	}
}

// TestOtherMalformedArguments proves a payload of the wrong shape is reported
// instead of being silently ignored.
func TestOtherMalformedArguments(t *testing.T) {
	d, _ := newTestDriver(t, http.NewServeMux())

	_, err := d.Other(context.Background(), model.OtherArgs{
		Method: "detail",
		Data:   map[string]any{"file_id": "not-a-number"},
	})
	if err == nil || !strings.Contains(err.Error(), "decode arguments") {
		t.Fatalf("error = %v, want a decoding error", err)
	}
}

func TestOtherSurfacesAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/info", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 401, "message": "token expired"})
	})
	d, _ := newTestDriver(t, mux)

	_, err := d.Other(context.Background(), model.OtherArgs{Method: "user_info"})
	if err == nil || !strings.Contains(err.Error(), "token expired") {
		t.Fatalf("error = %v, want the api message", err)
	}
	var apiErr *pan123.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want the sdk error to stay unwrappable", err)
	}
}

// TestOtherHandlersAreRegistered guards the dispatch table against a handler
// silently disappearing.
func TestOtherHandlersAreRegistered(t *testing.T) {
	want := []string{
		"user_info", "detail", "infos", "trash", "recover", "recover_to",
		"share_create", "share_list", "share_update",
		"share_create_paid", "share_list_paid", "share_update_paid",
		"direct_link_enable", "direct_link_disable", "direct_link_url",
		"direct_link_refresh_cache", "direct_link_traffic", "direct_link_offline_logs",
		"direct_link_ip_blacklist", "direct_link_ip_blacklist_update", "direct_link_ip_blacklist_switch",
		"offline_download", "offline_process",
		"transcode_folder_info", "transcode_cloud_video_files", "transcode_space_files",
		"transcode_upload_from_cloud_disk", "transcode_resolutions", "transcode_video",
		"transcode_records", "transcode_results", "transcode_list", "transcode_delete",
		"transcode_download_original", "transcode_download_m3u8", "transcode_download_ts",
		"transcode_download_all",
		"oss_mkdir", "oss_list", "oss_detail", "oss_move", "oss_delete",
		"oss_copy_from_disk", "oss_copy_process", "oss_copy_fail_list",
		"oss_offline_download", "oss_offline_process",
	}
	registered := map[string]bool{}
	for _, name := range OtherMethods() {
		registered[name] = true
	}
	for _, name := range want {
		if !registered[name] {
			t.Errorf("method %q is not registered", name)
		}
	}
}

// toMap re-encodes an Other result the way the API layer would.
func toMap(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal result %s: %v", raw, err)
	}
	return out
}

func jsonEqual(got, want interface{}) bool {
	gotRaw, err := json.Marshal(got)
	if err != nil {
		return false
	}
	wantRaw, err := json.Marshal(want)
	if err != nil {
		return false
	}
	return string(gotRaw) == string(wantRaw)
}
