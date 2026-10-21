package _123Open

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	pan123 "github.com/okatu-loli/go-123pan"
)

// listPageSize is the maximum page size accepted by the v2 file list API.
const listPageSize = 100

// List walks the whole directory, following the lastFileId cursor until the
// API reports the last page. Trashed entries are still returned by the API and
// are filtered out here.
func (d *Open123) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	if err := d.ensureToken(ctx); err != nil {
		return nil, err
	}
	parentFileID, err := parseFileID(dir.GetID())
	if err != nil {
		return nil, err
	}

	req := &pan123.FileListRequest{ParentFileID: parentFileID, Limit: listPageSize}
	var objs []model.Obj
	for {
		page, err := d.client.File.List(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("list %s failed: %w", dir.GetPath(), err)
		}
		for _, f := range page.FileList {
			if f.Trashed != 0 {
				continue
			}
			objs = append(objs, fileToObj(f))
		}
		// -1 marks the last page; an unchanged cursor would loop forever
		if page.LastFileID == -1 || len(page.FileList) == 0 || page.LastFileID == req.LastFileID {
			return objs, nil
		}
		req.LastFileID = page.LastFileID
	}
}

// Link resolves a temporary download URL. It uses the download API by default
// and the direct link service when UseDirectLink is enabled; the resulting URL
// is signed when a private key is configured.
func (d *Open123) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if file.IsDir() {
		return nil, errs.LinkIsDir
	}
	if err := d.ensureToken(ctx); err != nil {
		return nil, err
	}
	fileID, err := parseFileID(file.GetID())
	if err != nil {
		return nil, err
	}

	var rawURL string
	if d.UseDirectLink {
		rawURL, err = d.client.Link.URL(ctx, fileID)
	} else {
		rawURL, err = d.client.File.DownloadInfo(ctx, fileID)
	}
	if err != nil {
		return nil, fmt.Errorf("get link of %s failed: %w", file.GetName(), err)
	}
	if rawURL == "" {
		return nil, errors.New("the server returned an empty download url")
	}

	if d.PrivateKey != "" {
		if d.UID == 0 {
			return nil, errors.New("uid is required when private_key is set")
		}
		validDuration := time.Duration(d.ValidDuration) * time.Minute
		if validDuration <= 0 {
			validDuration = 30 * time.Minute
		}
		rawURL, err = SignURL(rawURL, d.PrivateKey, d.UID, validDuration)
		if err != nil {
			return nil, fmt.Errorf("sign direct link failed: %w", err)
		}
	}

	return &model.Link{URL: rawURL}, nil
}
