package webdav

import (
	"testing"

	"github.com/alist-org/alist/v3/internal/model"
)

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		raw      string
		want     string
	}{
		{
			name:     "non-root base root request",
			basePath: "/webdav",
			raw:      "/",
			want:     "/webdav",
		},
		{
			name:     "non-root base relative child",
			basePath: "/webdav",
			raw:      "/cc-switch-sync",
			want:     "/webdav/cc-switch-sync",
		},
		{
			name:     "non-root base already-qualified child",
			basePath: "/webdav",
			raw:      "/webdav/cc-switch-sync",
			want:     "/webdav/cc-switch-sync",
		},
		{
			name:     "non-root base itself",
			basePath: "/webdav",
			raw:      "/webdav",
			want:     "/webdav",
		},
		{
			name:     "root base",
			basePath: "/",
			raw:      "/cc-switch-sync",
			want:     "/cc-switch-sync",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePath(&model.User{BasePath: tt.basePath}, tt.raw)
			if err != nil {
				t.Fatalf("ResolvePath(%q, %q) error: %v", tt.basePath, tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("ResolvePath(%q, %q) = %q, want %q", tt.basePath, tt.raw, got, tt.want)
			}
		})
	}
}
