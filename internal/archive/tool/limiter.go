package tool

import (
	"errors"
	"io"
	"sync/atomic"
)

var ErrExtractSizeExceeded = errors.New("total size of decompressed files exceeds the limit")

// SizeLimiter limits the total bytes written by one decompress task.
// A non-positive max means no limit.
type SizeLimiter struct {
	remain  int64
	limited bool
}

func NewSizeLimiter(max int64) *SizeLimiter {
	if max <= 0 {
		return &SizeLimiter{}
	}
	return &SizeLimiter{remain: max, limited: true}
}

func (l *SizeLimiter) WrapWriter(w io.Writer) io.Writer {
	if l == nil || !l.limited {
		return w
	}
	return &limitedWriter{w: w, limiter: l}
}

type limitedWriter struct {
	w       io.Writer
	limiter *SizeLimiter
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if atomic.AddInt64(&lw.limiter.remain, -int64(len(p))) < 0 {
		return 0, ErrExtractSizeExceeded
	}
	return lw.w.Write(p)
}
