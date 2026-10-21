package lanzou

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alist-org/alist/v3/internal/conf"
)

// The login endpoint may be fronted by the acw_sc__v2 anti-bot challenge just
// like any other lanzou page. Login must solve the challenge and retry with
// the computed cookie instead of failing with "login err: <challenge html>".
func TestLoginSolvesAcwScV2Challenge(t *testing.T) {
	if conf.Conf == nil {
		conf.Conf = conf.DefaultConfig()
	}
	const arg1 = "F921C7A4073A625268C31B50EAA8769EABA0927B"
	wantAcw, err := CalcAcwScV2(fmt.Sprintf("arg1='%s'", arg1))
	if err != nil {
		t.Fatalf("CalcAcwScV2: %v", err)
	}

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if c, err := r.Cookie("acw_sc__v2"); err != nil || c.Value != wantAcw {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><script>var arg1='%s';</script></html>", arg1)
			return
		}
		if got := r.FormValue("uid"); got != "user1" {
			t.Errorf("uid = %q, want user1", got)
		}
		http.SetCookie(w, &http.Cookie{Name: "phpdisk_info", Value: "abc"})
		fmt.Fprint(w, `{"zt":1,"info":"成功登录","id":1}`)
	}))
	defer srv.Close()

	oldURL := loginURL
	loginURL = srv.URL + "/mlogin.php"
	defer func() { loginURL = oldURL }()

	d := &LanZou{}
	d.Account = "user1"
	d.Password = "pass1"

	cookies, err := d.Login()
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected challenge retry, got %d call(s)", calls)
	}
	var found bool
	for _, c := range cookies {
		if c.Name == "phpdisk_info" && c.Value == "abc" {
			found = true
		}
	}
	if !found {
		t.Fatalf("phpdisk_info cookie missing in %v", cookies)
	}
	if d.Cookie == "" {
		t.Fatal("d.Cookie not set after login")
	}
}
