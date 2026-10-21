package s3

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Providers like CSTCloud data capsule reject requests whose User-Agent does
// not contain the app type the AccessKey was created for. The configured UA
// must reach every client built from the session (API client, uploader).
func TestInitSessionAppliesUserAgent(t *testing.T) {
	d := &S3{}
	d.Endpoint = "https://s3.example.com"
	d.Region = "us-east-1"
	d.AccessKeyID = "ak"
	d.SecretAccessKey = "sk"
	d.UserAgent = "rclone/v1.65.0"

	if err := d.initSession(); err != nil {
		t.Fatalf("initSession: %v", err)
	}

	client := s3.New(d.Session)
	req, _ := client.ListObjectsV2Request(&s3.ListObjectsV2Input{Bucket: aws.String("b")})
	if err := req.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := req.HTTPRequest.Header.Get("User-Agent"); got != "rclone/v1.65.0" {
		t.Fatalf("User-Agent = %q, want %q", got, "rclone/v1.65.0")
	}
}

// Without the option the SDK default UA must remain untouched.
func TestInitSessionDefaultUserAgent(t *testing.T) {
	d := &S3{}
	d.Endpoint = "https://s3.example.com"
	d.Region = "us-east-1"
	d.AccessKeyID = "ak"
	d.SecretAccessKey = "sk"

	if err := d.initSession(); err != nil {
		t.Fatalf("initSession: %v", err)
	}

	client := s3.New(d.Session)
	req, _ := client.ListObjectsV2Request(&s3.ListObjectsV2Input{Bucket: aws.String("b")})
	if err := req.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := req.HTTPRequest.Header.Get("User-Agent"); !strings.Contains(got, "aws-sdk-go") {
		t.Fatalf("User-Agent = %q, want SDK default containing aws-sdk-go", got)
	}
}
