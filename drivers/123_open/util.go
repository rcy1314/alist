package _123Open

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	pan123 "github.com/okatu-loli/go-123pan"
)

// timeLayout is the timestamp format returned by the open platform, expressed
// in Beijing time.
const timeLayout = "2006-01-02 15:04:05"

// cst is the timezone the open platform reports its timestamps in.
var cst = time.FixedZone("CST", 8*3600)

// parseFileID turns an AList object ID into the numeric file ID used by the
// API. The root directory is 0.
func parseFileID(id string) (int64, error) {
	id = strings.TrimSpace(id)
	if id == "" || id == "root" {
		return 0, nil
	}
	fileID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid file id %q: %w", id, err)
	}
	return fileID, nil
}

// parseTime parses an open platform timestamp, returning the zero time when it
// is missing or malformed.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.ParseInLocation(timeLayout, s, cst)
	if err != nil {
		return time.Time{}
	}
	return t
}

// fileToObj maps a v2 file list entry onto an AList object. The Etag exposed by
// the API is the file MD5, so it is carried over as the object hash.
func fileToObj(f pan123.FileInfo) model.Obj {
	obj := &model.Object{
		ID:       strconv.FormatInt(f.FileID, 10),
		Name:     f.Filename,
		Size:     f.Size,
		Modified: parseTime(f.UpdateAt),
		Ctime:    parseTime(f.CreateAt),
		IsFolder: f.Type == 1,
	}
	if f.Etag != "" && !obj.IsFolder {
		obj.HashInfo = utils.NewHashInfo(utils.MD5, f.Etag)
	}
	return obj
}
