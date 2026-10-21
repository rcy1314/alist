package tool

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestSizeLimiterExceeded(t *testing.T) {
	l := NewSizeLimiter(10)
	var buf bytes.Buffer
	_, err := io.Copy(l.WrapWriter(&buf), strings.NewReader("0123456789abcdef"))
	if !errors.Is(err, ErrExtractSizeExceeded) {
		t.Fatalf("expected ErrExtractSizeExceeded, got %v", err)
	}
}

func TestSizeLimiterSharedAcrossWriters(t *testing.T) {
	l := NewSizeLimiter(10)
	var a, b bytes.Buffer
	if _, err := l.WrapWriter(&a).Write([]byte("123456")); err != nil {
		t.Fatalf("first write should pass, got %v", err)
	}
	if _, err := l.WrapWriter(&b).Write([]byte("123456")); !errors.Is(err, ErrExtractSizeExceeded) {
		t.Fatalf("expected ErrExtractSizeExceeded, got %v", err)
	}
}

func TestSizeLimiterUnlimited(t *testing.T) {
	l := NewSizeLimiter(0)
	var buf bytes.Buffer
	n, err := io.Copy(l.WrapWriter(&buf), strings.NewReader("0123456789abcdef"))
	if err != nil || n != 16 {
		t.Fatalf("unlimited limiter should pass all data, n=%d err=%v", n, err)
	}
}
