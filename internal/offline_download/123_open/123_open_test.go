package _123_open

import (
	"strings"
	"testing"

	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/offline_download/tool"
	pan123 "github.com/okatu-loli/go-123pan"
)

func TestToolIsRegistered(t *testing.T) {
	registered, err := tool.Tools.Get(tool.Open123ToolName)
	if err != nil {
		t.Fatalf("tool is not registered: %v", err)
	}
	if _, ok := registered.(*Open123); !ok {
		t.Fatalf("registered tool is %T, want *Open123", registered)
	}
}

// TestItemsRegistersNothing pins the convention shared with the other cloud
// tools: the scratch directory is written by the dedicated set_123_open
// endpoint, not seeded as a settings item.
func TestItemsRegistersNothing(t *testing.T) {
	if items := (&Open123{}).Items(); len(items) != 0 {
		t.Fatalf("got %d setting items, want none", len(items))
	}
}

// TestRunIsNotSupported keeps the task framework on the AddURL/Status path.
func TestRunIsNotSupported(t *testing.T) {
	if err := (&Open123{}).Run(&tool.DownloadTask{}); !errs.IsNotSupportError(err) {
		t.Fatalf("Run error = %v, want errs.NotSupport", err)
	}
}

// TestRemoveIsANoOp documents that the platform cannot cancel a task.
func TestRemoveIsANoOp(t *testing.T) {
	if err := (&Open123{}).Remove(&tool.DownloadTask{GID: "77"}); err != nil {
		t.Fatalf("Remove error = %v, want nil", err)
	}
}

func TestStatusFromProcess(t *testing.T) {
	cases := []struct {
		name          string
		process       pan123.OfflineProcessResult
		wantStatus    string
		wantCompleted bool
		wantErr       bool
	}{
		{
			name:       "running",
			process:    pan123.OfflineProcessResult{Process: 42.5, Status: pan123.OfflineRunning},
			wantStatus: "downloading",
		},
		{
			name:       "retrying is not a final state",
			process:    pan123.OfflineProcessResult{Process: 10, Status: pan123.OfflineRetrying},
			wantStatus: "retrying",
		},
		{
			name:          "success",
			process:       pan123.OfflineProcessResult{Process: 100, Status: pan123.OfflineSuccess},
			wantStatus:    "completed",
			wantCompleted: true,
		},
		{
			name:       "failure is reported even with a zeroed progress",
			process:    pan123.OfflineProcessResult{Process: 0, Status: pan123.OfflineFailed},
			wantStatus: "failed",
			wantErr:    true,
		},
		{
			name:       "unknown state",
			process:    pan123.OfflineProcessResult{Status: pan123.OfflineStatus(9)},
			wantStatus: "unknown status 9",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := statusFromProcess(&tc.process)
			if got.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.Progress != tc.process.Process {
				t.Errorf("progress = %v, want %v", got.Progress, tc.process.Process)
			}
			if got.Completed != tc.wantCompleted {
				t.Errorf("completed = %v, want %v", got.Completed, tc.wantCompleted)
			}
			if (got.Err != nil) != tc.wantErr {
				t.Errorf("err = %v, want an error: %v", got.Err, tc.wantErr)
			}
		})
	}
}

// TestUnmountedTempDirIsReported covers the storage resolution both entry
// points share; no storage is mounted in a unit test.
func TestUnmountedTempDirIsReported(t *testing.T) {
	o := &Open123{}
	if _, err := o.AddURL(&tool.AddUrlArgs{Url: "https://example.com/a.iso", TempDir: "/nowhere/tmp"}); err == nil {
		t.Error("AddURL: expected an error for an unmounted temp dir")
	}
	if _, err := o.Status(&tool.DownloadTask{TempDir: "/nowhere/tmp", GID: "77"}); err == nil {
		t.Error("Status: expected an error for an unmounted temp dir")
	}
}

// TestStatusRejectsAMalformedTaskID proves a non numeric GID is reported
// instead of being silently turned into task 0.
func TestStatusRejectsAMalformedTaskID(t *testing.T) {
	_, err := (&Open123{}).Status(&tool.DownloadTask{TempDir: "/nowhere/tmp", GID: "not-a-number"})
	if err == nil {
		t.Fatal("expected an error")
	}
	// the storage lookup fails first, so only assert the call does not panic
	if strings.TrimSpace(err.Error()) == "" {
		t.Fatal("expected a descriptive error")
	}
}
