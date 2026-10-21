package _123Open

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/stream"
	"github.com/alist-org/alist/v3/pkg/utils"
	pan123 "github.com/okatu-loli/go-123pan"
	"golang.org/x/sync/errgroup"
)

// completePollInterval is how long to wait between two upload completion
// checks; the API asks for at least one second.
var completePollInterval = time.Second

// completeMaxPolls bounds how long an upload waits for the server to finish
// merging before it gives up.
var completeMaxPolls = 60

// Put uploads a file with the v2 flow: create (which may hit the server-side
// instant upload), upload every slice, then wait for the server to merge them.
func (d *Open123) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	if err := d.ensureToken(ctx); err != nil {
		return nil, err
	}
	if up == nil {
		up = func(float64) {}
	}
	parentFileID, err := parseFileID(dstDir.GetID())
	if err != nil {
		return nil, err
	}

	// the create call needs the whole file MD5; reuse the stream's own hash
	// when it has one, otherwise cache the stream and hash it on the way
	etag := file.GetHash().GetHash(utils.MD5)
	if len(etag) != utils.MD5.Width {
		_, etag, err = stream.CacheFullInTempFileAndHash(file, utils.MD5)
		if err != nil {
			return nil, fmt.Errorf("calculate md5 of %s failed: %w", file.GetName(), err)
		}
	}

	size := file.GetSize()
	created, err := d.client.Upload.Create(ctx, &pan123.UploadCreateRequest{
		ParentFileID: parentFileID,
		Filename:     file.GetName(),
		Etag:         etag,
		Size:         size,
	})
	if err != nil {
		return nil, fmt.Errorf("create upload of %s failed: %w", file.GetName(), err)
	}
	// instant upload: the server already holds this content
	if created.Reuse {
		up(100)
		return uploadedObj(created.FileID, file, etag), nil
	}
	if created.PreuploadID == "" {
		return nil, errors.New("the server returned no preuploadID")
	}

	servers := created.Servers
	if len(servers) == 0 {
		servers, err = d.client.Upload.Domains(ctx)
		if err != nil {
			return nil, fmt.Errorf("get upload domains failed: %w", err)
		}
	}
	if len(servers) == 0 {
		return nil, errors.New("the server returned no upload domain")
	}
	sliceSize := created.SliceSize
	if sliceSize <= 0 {
		return nil, fmt.Errorf("the server returned an invalid slice size %d", sliceSize)
	}

	// slices are uploaded concurrently, so random access to the content is
	// required
	tmpF, err := file.CacheFullInTempFile()
	if err != nil {
		return nil, err
	}

	sliceCount := (size + sliceSize - 1) / sliceSize
	if sliceCount == 0 {
		sliceCount = 1
	}
	uploadThread := d.UploadThread
	if uploadThread <= 0 {
		uploadThread = 3
	}
	if int64(uploadThread) > sliceCount {
		uploadThread = int(sliceCount)
	}

	progress := &uploadProgress{total: size, up: up}
	threadG, uploadCtx := errgroup.WithContext(ctx)
	threadG.SetLimit(uploadThread)
	for i := int64(0); i < sliceCount; i++ {
		if utils.IsCanceled(uploadCtx) {
			break
		}
		sliceNo := i + 1
		offset := i * sliceSize
		length := min(sliceSize, size-offset)
		threadG.Go(func() error {
			sliceMD5, err := utils.HashReader(utils.MD5, io.NewSectionReader(tmpF, offset, length))
			if err != nil {
				return fmt.Errorf("calculate md5 of slice %d failed: %w", sliceNo, err)
			}
			reader := &progressReader{
				Reader:   io.NewSectionReader(tmpF, offset, length),
				progress: progress,
			}
			server := servers[int(i)%len(servers)]
			err = d.client.Upload.UploadSlice(uploadCtx, server, created.PreuploadID, sliceNo, sliceMD5,
				driver.NewLimitedUploadStream(uploadCtx, reader))
			if err != nil {
				return fmt.Errorf("upload slice %d failed: %w", sliceNo, err)
			}
			return nil
		})
	}
	if err := threadG.Wait(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// The server merges and verifies the slices asynchronously. While it is
	// working it may answer either with completed=false or with an
	// undocumented business error (observed: code 20103, "文件正在..."), so
	// both are treated as "not finished yet" and the last one is only
	// reported once the attempts run out.
	var lastErr error
	for i := 0; i < completeMaxPolls; i++ {
		res, err := d.client.Upload.Complete(ctx, created.PreuploadID)
		switch {
		case err != nil:
			lastErr = err
		case res.Completed && res.FileID != 0:
			up(100)
			return uploadedObj(res.FileID, file, etag), nil
		default:
			lastErr = nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(completePollInterval):
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("complete upload of %s failed: %w", file.GetName(), lastErr)
	}
	return nil, fmt.Errorf("complete upload of %s timed out while the server was merging the slices", file.GetName())
}

// uploadedObj describes the file the server created for an upload.
func uploadedObj(fileID int64, file model.FileStreamer, etag string) model.Obj {
	return &model.Object{
		ID:       strconv.FormatInt(fileID, 10),
		Name:     file.GetName(),
		Size:     file.GetSize(),
		Modified: time.Now(),
		Ctime:    time.Now(),
		HashInfo: utils.NewHashInfo(utils.MD5, etag),
	}
}

// uploadProgress accumulates the bytes uploaded by all concurrent slices and
// forwards the overall percentage. Reporting is serialized because the callback
// is not required to be safe for concurrent use.
type uploadProgress struct {
	total int64
	done  int64
	mu    sync.Mutex
	up    driver.UpdateProgress
}

func (p *uploadProgress) add(n int64) {
	if p.total <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done += n
	percentage := float64(p.done) / float64(p.total) * 100
	if percentage > 100 {
		percentage = 100
	}
	p.up(percentage)
}

// progressReader reports every byte read to the shared upload progress.
type progressReader struct {
	io.Reader
	progress *uploadProgress
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > 0 {
		r.progress.add(int64(n))
	}
	return n, err
}
