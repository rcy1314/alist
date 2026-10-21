package _123_open

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	_123Open "github.com/alist-org/alist/v3/drivers/123_open"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/offline_download/tool"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/internal/setting"
	pan123 "github.com/okatu-loli/go-123pan"
	log "github.com/sirupsen/logrus"
)

// errWrongStorage is reported when the resolved storage is not a 123 Open one:
// the open platform can only download into the account that owns the token.
var errWrongStorage = errors.New("123 Open offline download only supports a 123 Open destination storage")

type Open123 struct{}

func (o *Open123) Name() string {
	return tool.Open123ToolName
}

// Items registers no settings: the scratch directory is written by the
// dedicated set_123_open endpoint, like the other cloud tools.
func (o *Open123) Items() []model.SettingItem {
	return nil
}

// Run reports NotSupport so the framework drives the task through AddURL and
// Status instead.
func (o *Open123) Run(task *tool.DownloadTask) error {
	return errs.NotSupport
}

func (o *Open123) Init() (string, error) {
	return "ok", nil
}

func (o *Open123) IsReady() bool {
	tempDir := setting.GetStr(conf.Open123TempDir)
	if tempDir == "" {
		return false
	}
	storage, _, err := op.GetStorageAndActualPath(tempDir)
	if err != nil {
		return false
	}
	_, ok := storage.(*_123Open.Open123)
	return ok
}

func (o *Open123) AddURL(args *tool.AddUrlArgs) (string, error) {
	storage, actualPath, err := op.GetStorageAndActualPath(args.TempDir)
	if err != nil {
		return "", err
	}
	driver, ok := storage.(*_123Open.Open123)
	if !ok {
		return "", errWrongStorage
	}

	ctx := context.Background()
	if err := op.MakeDir(ctx, storage, actualPath); err != nil {
		return "", err
	}
	parentDir, err := op.GetUnwrap(ctx, storage, actualPath)
	if err != nil {
		return "", err
	}

	taskID, err := driver.OfflineDownload(ctx, args.Url, parentDir, "")
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(taskID, 10), nil
}

// Remove is a no-op: the open platform exposes no way to cancel or delete an
// offline download task, so a cancelled task keeps running on 123's side.
func (o *Open123) Remove(task *tool.DownloadTask) error {
	log.Warnf("123 Open cannot cancel offline download task %s, it keeps running remotely", task.GID)
	return nil
}

func (o *Open123) Status(task *tool.DownloadTask) (*tool.Status, error) {
	storage, _, err := op.GetStorageAndActualPath(task.TempDir)
	if err != nil {
		return nil, err
	}
	driver, ok := storage.(*_123Open.Open123)
	if !ok {
		return nil, errWrongStorage
	}
	taskID, err := strconv.ParseInt(task.GID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid task id %q: %w", task.GID, err)
	}

	process, err := driver.OfflineProcess(context.Background(), taskID)
	if err != nil {
		return nil, err
	}
	return statusFromProcess(process), nil
}

// statusFromProcess maps a task progress report onto the framework's status.
// The platform reports no size, only a percentage, so TotalBytes stays zero.
func statusFromProcess(process *pan123.OfflineProcessResult) *tool.Status {
	s := &tool.Status{
		Progress: process.Process,
		Status:   offlineStatusText(process.Status),
	}
	switch process.Status {
	case pan123.OfflineSuccess:
		s.Completed = true
	case pan123.OfflineFailed:
		// a failed task zeroes its progress, so only the state is meaningful
		s.Err = errors.New("offline download failed")
	}
	return s
}

func offlineStatusText(status pan123.OfflineStatus) string {
	switch status {
	case pan123.OfflineRunning:
		return "downloading"
	case pan123.OfflineFailed:
		return "failed"
	case pan123.OfflineSuccess:
		return "completed"
	case pan123.OfflineRetrying:
		return "retrying"
	default:
		return fmt.Sprintf("unknown status %d", int(status))
	}
}

var _ tool.Tool = (*Open123)(nil)

func init() {
	tool.Tools.Add(&Open123{})
}
