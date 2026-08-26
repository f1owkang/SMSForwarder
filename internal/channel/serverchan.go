package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

const serverchanTemplate = "https://sctapi.ftqq.com/%s.send"

type ServerChan struct {
	SendKey  string
	Endpoint string
	HC       *HTTPClient
}

func NewServerChan(sendKey string, hc *HTTPClient) *ServerChan {
	return &ServerChan{SendKey: sendKey, Endpoint: fmt.Sprintf(serverchanTemplate, sendKey), HC: hc}
}

func (s *ServerChan) Name() string { return "serverchan" }

func (s *ServerChan) Send(ctx context.Context, m Message) error {
	form := url.Values{}
	form.Set("title", Title(m))
	form.Set("desp", Body(m))
	resp, err := s.HC.PostFormResp(ctx, s.Endpoint, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("响应解析失败: %w", err)
	}
	if out.Code != 0 {
		return fmt.Errorf("serverchan 返回 code=%d msg=%s", out.Code, out.Message)
	}
	return nil
}
