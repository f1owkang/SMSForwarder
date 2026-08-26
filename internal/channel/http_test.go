package channel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient() (*HTTPClient, *[]time.Duration) {
	var sleeps []time.Duration
	h := NewHTTPClient()
	h.sleep = func(d time.Duration) { sleeps = append(sleeps, d) }
	return h, &sleeps
}

func TestPostFormSuccessNoRetry(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		if string(body) != "a=1&b=2" && string(body) != "b=2&a=1" {
			t.Errorf("form 内容错误: %q", body)
		}
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	h, sleeps := newTestClient()
	resp, err := h.PostFormResp(context.Background(), srv.URL, url.Values{"a": {"1"}, "b": {"2"}})
	if err != nil {
		t.Fatalf("不应失败: %v", err)
	}
	resp.Body.Close()
	if hits.Load() != 1 || len(*sleeps) != 0 {
		t.Fatalf("成功不应重试: hits=%d sleeps=%v", hits.Load(), *sleeps)
	}
}

func TestRetryOn500ThenSuccess(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	h, sleeps := newTestClient()
	resp, err := h.PostJSONResp(context.Background(), srv.URL, map[string]string{"x": "y"}, nil)
	if err != nil {
		t.Fatalf("第三次应成功: %v", err)
	}
	resp.Body.Close()
	if hits.Load() != 3 {
		t.Fatalf("应尝试 3 次: %d", hits.Load())
	}
	if (*sleeps)[0] != time.Second || (*sleeps)[1] != 2*time.Second {
		t.Fatalf("退避应为 1s、2s: %v", *sleeps)
	}
}

func TestPersistentRetryableStatusFails(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	h, _ := newTestClient()
	if _, err := h.PostFormResp(context.Background(), srv.URL, nil); err == nil {
		t.Fatal("持续 502 应返回错误")
	}
	if hits.Load() != 3 {
		t.Fatalf("应尝试 3 次: %d", hits.Load())
	}
}

func TestBusinessErrorNotRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	h, sleeps := newTestClient()
	resp, err := h.PostJSONResp(context.Background(), srv.URL, nil, nil)
	if err != nil || resp.StatusCode != 400 {
		t.Fatalf("业务错误应原样返回给调用方判定: %v %d", err, resp.StatusCode)
	}
	resp.Body.Close()
	if hits.Load() != 1 || len(*sleeps) != 0 {
		t.Fatalf("非 500/502/504 不应重试: hits=%d sleeps=%v", hits.Load(), *sleeps)
	}
}

func TestNetworkErrorRetriesThenFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	h, _ := newTestClient()
	if _, err := h.PostFormResp(context.Background(), url, nil); err == nil {
		t.Fatal("网络错误最终应失败")
	}
}

func TestRetryReplaysBodyIdentically(t *testing.T) {
	var hits atomic.Int32
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		if hits.Add(1) <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	h, _ := newTestClient()
	resp, err := h.PostFormResp(context.Background(), srv.URL, url.Values{"a": {"1"}, "b": {"2"}})
	if err != nil {
		t.Fatalf("第三次应成功: %v", err)
	}
	resp.Body.Close()
	mu.Lock()
	defer mu.Unlock()
	if hits.Load() != 3 || len(bodies) != 3 {
		t.Fatalf("应尝试 3 次: hits=%d bodies=%d", hits.Load(), len(bodies))
	}
	for i, b := range bodies {
		if b != "a=1&b=2" {
			t.Errorf("第 %d 次请求体被改变: %q", i+1, b)
		}
	}
}

func TestTitleBodyHelpers(t *testing.T) {
	m := Message{Number: "10086", Text: "hi", Keyword: "", Timestamp: "t"}
	if Title(m) != "10086" {
		t.Fatal("Keyword 为空时 Title 应用 Number")
	}
	m.Keyword = "验证码【1234】"
	if Title(m) != "验证码【1234】" {
		t.Fatal("Title 应优先 Keyword")
	}
	if Body(m) != "10086\nhi\nt" {
		t.Fatalf("Body 格式错误: %q", Body(m))
	}
}
