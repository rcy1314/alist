package _123Open

import (
	"context"
	"errors"
	"fmt"

	"github.com/alist-org/alist/v3/internal/model"
	pan123 "github.com/okatu-loli/go-123pan"
)

// errOfflineRootDir is reported when an offline download is aimed at the root
// of the storage: the open platform rejects the root directory and silently
// drops such tasks into its own "来自:离线下载" folder instead.
var errOfflineRootDir = errors.New("123 offline download cannot target the root directory, pick a sub directory")

// OfflineDownload submits an offline download task that saves the URL into
// parentDir, and returns the task id to poll with OfflineProcess. It is used by
// the offline-download tool, which lives outside this package.
func (d *Open123) OfflineDownload(ctx context.Context, url string, parentDir model.Obj, fileName string) (int64, error) {
	if err := d.ensureToken(ctx); err != nil {
		return 0, err
	}
	dirID, err := parseFileID(parentDir.GetID())
	if err != nil {
		return 0, err
	}
	if dirID == 0 {
		return 0, errOfflineRootDir
	}
	taskID, err := d.client.Offline.Download(ctx, &pan123.OfflineDownloadRequest{
		URL:      url,
		FileName: fileName,
		DirID:    dirID,
	})
	if err != nil {
		return 0, fmt.Errorf("add offline download task failed: %w", err)
	}
	return taskID, nil
}

// OfflineProcess reports the progress and state of an offline download task.
func (d *Open123) OfflineProcess(ctx context.Context, taskID int64) (*pan123.OfflineProcessResult, error) {
	if err := d.ensureToken(ctx); err != nil {
		return nil, err
	}
	res, err := d.client.Offline.Process(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("query offline download task %d failed: %w", taskID, err)
	}
	return res, nil
}
