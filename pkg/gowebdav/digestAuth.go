package gowebdav

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DigestAuth structure holds our credentials
type DigestAuth struct {
	user        string
	pw          string
	digestParts map[string]string
}

// Type identifies the DigestAuthenticator
func (d *DigestAuth) Type() string {
	return "DigestAuth"
}

// User holds the DigestAuth username
func (d *DigestAuth) User() string {
	return d.user
}

// Pass holds the DigestAuth password
func (d *DigestAuth) Pass() string {
	return d.pw
}

// Authorize the current request
func (d *DigestAuth) Authorize(req *http.Request, method string, path string) {
	d.digestParts["uri"] = path
	d.digestParts["method"] = method
	d.digestParts["username"] = d.user
	d.digestParts["password"] = d.pw
	req.Header.Set("Authorization", getDigestAuthorization(d.digestParts))
}

func digestParts(resp *http.Response) map[string]string {
	result := map[string]string{}
	if len(resp.Header["Www-Authenticate"]) > 0 {
		wantedHeaders := []string{"nonce", "realm", "qop", "opaque", "algorithm", "entityBody"}
		responseHeaders := strings.Split(resp.Header["Www-Authenticate"][0], ",")
		for _, r := range responseHeaders {
			for _, w := range wantedHeaders {
				if strings.Contains(r, w) {
					result[w] = strings.Trim(
						strings.SplitN(r, `=`, 2)[1],
						`"`,
					)
				}
			}
		}
	}
	return result
}

func isStaleDigestChallenge(resp *http.Response) bool {
	for _, header := range resp.Header.Values("WWW-Authenticate") {
		for _, directive := range strings.Split(header, ",") {
			directive = strings.TrimSpace(directive)
			if len(directive) >= len("Digest ") && strings.EqualFold(directive[:len("Digest ")], "Digest ") {
				directive = strings.TrimSpace(directive[len("Digest "):])
			}
			name, value, ok := strings.Cut(directive, "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(name), "stale") {
				continue
			}
			value = strings.Trim(strings.TrimSpace(value), `"`)
			if strings.EqualFold(value, "true") {
				return true
			}
		}
	}
	return false
}

func getMD5(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getCnonce() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return fmt.Sprintf("%x", b)[:16]
}

func getDigestAuthorization(digestParts map[string]string) string {
	d := digestParts
	// These are the correct ha1 and ha2 for qop=auth. We should probably check for other types of qop.

	var (
		ha1        string
		ha2        string
		nonceCount = 00000001
		cnonce     = getCnonce()
		response   string
	)

	// 'ha1' value depends on value of "algorithm" field
	switch d["algorithm"] {
	case "MD5", "":
		ha1 = getMD5(d["username"] + ":" + d["realm"] + ":" + d["password"])
	case "MD5-sess":
		ha1 = getMD5(
			fmt.Sprintf("%s:%v:%s",
				getMD5(d["username"]+":"+d["realm"]+":"+d["password"]),
				nonceCount,
				cnonce,
			),
		)
	}

	// 'ha2' value depends on value of "qop" field
	switch d["qop"] {
	case "auth", "":
		ha2 = getMD5(d["method"] + ":" + d["uri"])
	case "auth-int":
		if d["entityBody"] != "" {
			ha2 = getMD5(d["method"] + ":" + d["uri"] + ":" + getMD5(d["entityBody"]))
		}
	}

	// 'response' value depends on value of "qop" field
	switch d["qop"] {
	case "":
		response = getMD5(
			fmt.Sprintf("%s:%s:%s",
				ha1,
				d["nonce"],
				ha2,
			),
		)
	case "auth", "auth-int":
		response = getMD5(
			fmt.Sprintf("%s:%s:%v:%s:%s:%s",
				ha1,
				d["nonce"],
				nonceCount,
				cnonce,
				d["qop"],
				ha2,
			),
		)
	}

	authorization := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", nc=%v, cnonce="%s", response="%s"`,
		d["username"], d["realm"], d["nonce"], d["uri"], nonceCount, cnonce, response)

	if d["qop"] != "" {
		authorization += fmt.Sprintf(`, qop=%s`, d["qop"])
	}

	if d["opaque"] != "" {
		authorization += fmt.Sprintf(`, opaque="%s"`, d["opaque"])
	}

	return authorization
}
