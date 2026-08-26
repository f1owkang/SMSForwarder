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

func testMsg() Message {
	return Message{Number: "10690001", Text: "您的订单已发货", Keyword: "订单、发货", Timestamp: "2026-08-26 12:00:00"}
}

func readBody(r *http.Request, n int) string {
	buf := make([]byte, n)
	c, _ := r.Body.Read(buf)
	return string(buf[:c])
}

func TestPushPlusRequestAndBizCode(t *testing.T) {
	var gotPath, gotForm string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotForm = readBody(r, 1024)
		io.WriteString(w, `{"code":200,"msg":"ok"}`)
	}))
	defer srv.Close()

	p := NewPushPlus("TOK", NewHTTPClient())
	p.Endpoint = srv.URL + "/send"
	if err := p.Send(context.Background(), testMsg()); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/send") ||
		!strings.Contains(gotForm, "token=TOK") ||
		!strings.Contains(gotForm, "title=10690001") ||
		!strings.Contains(gotForm, "content=") {
		t.Fatalf("请求不符: path=%q form=%q", gotPath, gotForm)
	}
	if !strings.Contains(gotForm, "%E8%AE%A2%E5%8D%95") {
		t.Fatalf("content 应含关键词: %q", gotForm)
	}
}

func TestPushPlusBizFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":900,"msg":"token错误"}`)
	}))
	defer srv.Close()
	p := NewPushPlus("TOK", NewHTTPClient())
	p.Endpoint = srv.URL
	err := p.Send(context.Background(), testMsg())
	if err == nil || !strings.Contains(err.Error(), "900") {
		t.Fatalf("业务码非 200 应报错: %v", err)
	}
}

func TestServerChan(t *testing.T) {
	var gotPath, gotForm string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotForm = readBody(r, 2048)
		io.WriteString(w, `{"code":0,"message":""}`)
	}))
	defer srv.Close()
	sc := NewServerChan("SCT123", NewHTTPClient())
	sc.Endpoint = srv.URL + "/SCT123.send"
	if err := sc.Send(context.Background(), testMsg()); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if !strings.Contains(gotPath, "SCT123.send") ||
		!strings.Contains(gotForm, "title=") || !strings.Contains(gotForm, "desp=") {
		t.Fatalf("请求不符: path=%q form=%q", gotPath, gotForm)
	}
}

func TestBarkDefaultServerAndPayload(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		io.WriteString(w, `{"code":200,"message":"success"}`)
	}))
	defer srv.Close()

	b := NewBark("", "DEVKEY", NewHTTPClient())
	if b.Endpoint != "https://api.day.app" {
		t.Fatalf("默认 server 错误: %q", b.Endpoint)
	}
	b.Endpoint = srv.URL
	if err := b.Send(context.Background(), testMsg()); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if !strings.Contains(gotPath, "/DEVKEY") || gotBody["title"] != "订单、发货" {
		t.Fatalf("请求不符: path=%q body=%v", gotPath, gotBody)
	}
}

func TestBarkBizFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":400,"message":"bad key"}`)
	}))
	defer srv.Close()
	b := NewBark(srv.URL, "K", NewHTTPClient())
	if err := b.Send(context.Background(), testMsg()); err == nil {
		t.Fatal("code 非 200 应报错")
	}
}
