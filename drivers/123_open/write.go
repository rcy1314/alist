package _123Open

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
	pan123 "github.com/okatu-loli/go-123pan"
)

// copyPollInterval is how long to wait between two async copy progress checks.
var copyPollInterval = time.Second

// MakeDir creates a directory and returns it, so the caller does not have to
// list the parent again.
func (d *Open123) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	if err := d.ensureToken(ctx); err != nil {
		return nil, err
	}
	parentFileID, err := parseFileID(parentDir.GetID())
	if err != nil {
		return nil, err
	}
	dirID, err := d.client.File.Mkdir(ctx, parentFileID, dirName)
	if err != nil {
		return nil, fmt.Errorf("mkdir %s failed: %w", dirName, err)
	}
	now := time.Now()
	return &model.Object{
		ID:       strconv.FormatInt(dirID, 10),
		Name:     dirName,
		IsFolder: true,
		Modified: now,
		Ctime:    now,
	}, nil
}

// Move moves a single object into dstDir. The file keeps its ID and name, so
// the updated object can be rebuilt locally.
func (d *Open123) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	if err := d.ensureToken(ctx); err != nil {
		return nil, err
	}
	srcID, err := parseFileID(srcObj.GetID())
	if err != nil {
		return nil, err
	}
	dstID, err := parseFileID(dstDir.GetID())
	if err != nil {
		return nil, err
	}
	if err := d.client.File.Move(ctx, []int64{srcID}, dstID); err != nil {
		return nil, fmt.Errorf("move %s failed: %w", srcObj.GetName(), err)
	}
	return copyObj(srcObj, srcObj.GetName(), srcObj.ModTime()), nil
}

// Rename renames a single object and returns it under its new name.
func (d *Open123) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	if err := d.ensureToken(ctx); err != nil {
		return nil, err
	}
	srcID, err := parseFileID(srcObj.GetID())
	if err != nil {
		return nil, err
	}
	if err := d.client.File.Rename(ctx, srcID, newName); err != nil {
		return nil, fmt.Errorf("rename %s failed: %w", srcObj.GetName(), err)
	}
	return copyObj(srcObj, newName, time.Now()), nil
}

// Copy copies an object into dstDir. The open platform only offers an async
// batch copy, whose task ID says nothing about the created file, so the plain
// (result-less) Copy interface is implemented and the task is polled until the
// server reports it done.
func (d *Open123) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	if err := d.ensureToken(ctx); err != nil {
		return err
	}
	srcID, err := parseFileID(srcObj.GetID())
	if err != nil {
		return err
	}
	dstID, err := parseFileID(dstDir.GetID())
	if err != nil {
		return err
	}
	taskID, err := d.client.File.AsyncCopy(ctx, []int64{srcID}, dstID)
	if err != nil {
		return fmt.Errorf("copy %s failed: %w", srcObj.GetName(), err)
	}
	return d.waitCopy(ctx, taskID)
}

// waitCopy polls an async copy task until it finishes, fails or the context is
// cancelled.
func (d *Open123) waitCopy(ctx context.Context, taskID int64) error {
	for {
		status, err := d.client.File.CopyProcess(ctx, taskID)
		if err != nil {
			return fmt.Errorf("query copy task %d failed: %w", taskID, err)
		}
		switch status {
		case pan123.CopyTaskDone:
			return nil
		case pan123.CopyTaskFailed:
			return fmt.Errorf("copy task %d failed", taskID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(copyPollInterval):
		}
	}
}

// Remove moves an object to the recycle bin.
func (d *Open123) Remove(ctx context.Context, obj model.Obj) error {
	if err := d.ensureToken(ctx); err != nil {
		return err
	}
	fileID, err := parseFileID(obj.GetID())
	if err != nil {
		return err
	}
	if err := d.client.File.Trash(ctx, []int64{fileID}); err != nil {
		return fmt.Errorf("remove %s failed: %w", obj.GetName(), err)
	}
	return nil
}

// copyObj rebuilds an object after an operation that keeps its identity.
func copyObj(src model.Obj, name string, modified time.Time) model.Obj {
	return &model.Object{
		ID:       src.GetID(),
		Path:     src.GetPath(),
		Name:     name,
		Size:     src.GetSize(),
		Modified: modified,
		Ctime:    src.CreateTime(),
		IsFolder: src.IsDir(),
		HashInfo: src.GetHash(),
	}
}
