package channel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelegramSend(t *testing.T) {
	var gotPath string
	var gotBody struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		io.WriteString(w, `{"ok":true,"result":{}}`)
	}))
	defer srv.Close()

	tg, err := NewTelegram("BOT", "42", "", NewHTTPClient())
	if err != nil {
		t.Fatal(err)
	}
	tg.Endpoint = srv.URL + "/botBOT/sendMessage"
	if err := tg.Send(context.Background(), testMsg()); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if !strings.Contains(gotPath, "/botBOT/sendMessage") {
		t.Fatalf("路径错误: %q", gotPath)
	}
	if gotBody.ChatID != "42" || !strings.HasPrefix(gotBody.Text, "订单、发货\n") {
		t.Fatalf("payload 错误: %+v", gotBody)
	}
}

func TestTelegramBizFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":false,"description":"Unauthorized"}`)
	}))
	defer srv.Close()
	tg, _ := NewTelegram("BAD", "1", "", NewHTTPClient())
	tg.Endpoint = srv.URL
	if err := tg.Send(context.Background(), testMsg()); err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("ok=false 应报错: %v", err)
	}
}

func TestTelegramInvalidProxy(t *testing.T) {
	if _, err := NewTelegram("B", "1", "://bad", NewHTTPClient()); err == nil {
		t.Fatal("非法代理应报错")
	}
}

func TestTelegramSOCKS5ProxyClient(t *testing.T) {
	tg, err := NewTelegram("B", "1", "socks5://127.0.0.1:1080", NewHTTPClient())
	if err != nil {
		t.Fatalf("socks5 代理构造失败: %v", err)
	}
	tr, ok := tg.HC.Base.Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatal("代理应体现在 Transport 上")
	}
	req, _ := http.NewRequest("GET", "https://api.telegram.org", nil)
	u, perr := tr.Proxy(req)
	if perr != nil || u.Scheme != "socks5" || u.Host != "127.0.0.1:1080" {
		t.Fatalf("代理解析错误: %v %v", u, perr)
	}
}

func TestWebhookPayloadAndStatus(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	wh := NewWebhook(srv.URL, NewHTTPClient())
	if err := wh.Send(context.Background(), testMsg()); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	for _, k := range []string{"number", "text", "keyword", "timestamp"} {
		if _, ok := gotBody[k]; !ok {
			t.Fatalf("payload 缺字段 %s: %v", k, gotBody)
		}
	}
	if gotBody["keyword"] != "订单、发货" {
		t.Fatalf("keyword 错误: %v", gotBody["keyword"])
	}
}

func TestWebhookNon2xxFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	h, _ := newTestClient()
	if _, err := h.PostJSONResp(context.Background(), srv.URL, nil, nil); err == nil {
		t.Fatal("500 属可重试状态，重试耗尽应报错")
	}
}
