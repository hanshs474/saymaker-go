package saymaker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func server(t *testing.T, final string, got *map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing bearer key")
		}
		if r.URL.Path == "/api/ai/generate" {
			_ = json.NewDecoder(r.Body).Decode(got)
			w.Write([]byte(`{"code":0,"data":{"id":"row1"}}`))
			return
		}
		w.Write([]byte(final))
	}))
}

func TestImagePrefersClean(t *testing.T) {
	var body map[string]any
	s := server(t, `{"code":0,"data":{"status":"success","images":["https://cdn/m.webp"],"cleanImages":["https://cdn/c.webp"]}}`, &body)
	defer s.Close()
	BaseURL = s.URL
	url, err := Generate(context.Background(), "x", Options{APIKey: "sk-test", PollEvery: time.Millisecond})
	if err != nil || url != "https://cdn/c.webp" {
		t.Fatal(url, err)
	}
	if body["model"] != ModelImage || body["provider"] != "kie" {
		t.Fatal(body)
	}
}

func TestVideoRoutes(t *testing.T) {
	var body map[string]any
	s := server(t, `{"code":0,"data":{"status":"success","videos":["https://cdn/v.mp4"]}}`, &body)
	defer s.Close()
	BaseURL = s.URL
	url, err := GenerateVideo(context.Background(), "x", VideoOptions{APIKey: "sk-test", Duration: 6, PollEvery: time.Millisecond})
	if err != nil || url != "https://cdn/v.mp4" {
		t.Fatal(url, err)
	}
	if body["provider"] != "vgenv" || body["scene"] != "text-to-video" {
		t.Fatal(body)
	}
}

func TestNeedsKey(t *testing.T) {
	t.Setenv("SAYMAKER_API_KEY", "")
	if _, err := Generate(context.Background(), "x", Options{}); !errors.Is(err, ErrAuth) {
		t.Fatal(err)
	}
}

func TestClassify(t *testing.T) {
	if !errors.Is(classify("insufficient credits"), ErrQuota) || !errors.Is(classify("invalid API key"), ErrAuth) {
		t.Fatal("classify")
	}
}
