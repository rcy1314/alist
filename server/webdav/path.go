package webdav

import (
	"strings"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
)

// ResolvePath normalizes the provided raw path and resolves it against the user's base path
// before delegating to the user-aware JoinPath permission checks.
func ResolvePath(user *model.User, raw string) (string, error) {
	cleaned := utils.FixAndCleanPath(raw)
	basePath := utils.FixAndCleanPath(user.BasePath)

	// WebDAV paths are normally relative to the user's base path, but some
	// clients include that base path in the request URL. Normalize both forms
	// to a path relative to BasePath before JoinPath applies it.
	if basePath != "/" && utils.IsSubPath(basePath, cleaned) {
		cleaned = strings.TrimPrefix(cleaned, basePath)
		if cleaned == "" {
			cleaned = "/"
		}
	}

	return user.JoinPath(cleaned)
}
