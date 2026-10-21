package handles

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/gin-gonic/gin"
)

func TestFsStreamAcceptsMissingContentLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/fs/put", http.NoBody)
	request.Header.Set("File-Path", "/test.txt")
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set("user", &model.User{BasePath: "/"})

	FsStream(context)

	if recorder.Code == http.StatusBadRequest || strings.Contains(recorder.Body.String(), "strconv.ParseInt") {
		t.Fatalf("missing Content-Length was rejected before upload handling: code=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestFsStreamRejectsInvalidContentLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/fs/put", http.NoBody)
	request.Header.Set("File-Path", "/test.txt")
	request.Header.Set("Content-Length", "invalid")
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set("user", &model.User{BasePath: "/"})

	FsStream(context)

	if !strings.Contains(recorder.Body.String(), `"code":400`) || !strings.Contains(recorder.Body.String(), "strconv.ParseInt") {
		t.Fatalf("invalid Content-Length was not rejected: code=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
