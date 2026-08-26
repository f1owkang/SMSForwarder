package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

const pushplusEndpoint = "https://www.pushplus.plus/send"

type PushPlus struct {
	Token    string
	Endpoint string
	HC       *HTTPClient
}

func NewPushPlus(token string, hc *HTTPClient) *PushPlus {
	return &PushPlus{Token: token, Endpoint: pushplusEndpoint, HC: hc}
}

func (p *PushPlus) Name() string { return "pushplus" }

func (p *PushPlus) Send(ctx context.Context, m Message) error {
	form := url.Values{}
	form.Set("token", p.Token)
	form.Set("title", Title(m))
	form.Set("content", m.Keyword+"\n\n"+Body(m))
	resp, err := p.HC.PostFormResp(ctx, p.Endpoint, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("响应解析失败: %w", err)
	}
	if out.Code != 200 {
		return fmt.Errorf("pushplus 返回 code=%d msg=%s", out.Code, out.Msg)
	}
	return nil
}
