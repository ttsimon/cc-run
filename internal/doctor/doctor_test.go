package doctor

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ttsimon/cc-run/internal/profile"
)

func TestCheck_可达返回ok(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	p := profile.Profile{Name: "x", Env: map[string]string{"ANTHROPIC_BASE_URL": srv.URL}}
	res := Check(p)
	if !res.OK {
		t.Errorf("可达应 OK: %+v", res)
	}
}

func TestCheck_无base_url标记跳过(t *testing.T) {
	p := profile.Profile{Name: "x", Env: map[string]string{}}
	res := Check(p)
	if res.OK || res.Detail == "" {
		t.Errorf("无 BASE_URL 应非 OK 且给原因: %+v", res)
	}
}
