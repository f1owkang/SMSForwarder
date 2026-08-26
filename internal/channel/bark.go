package channel

import (
	"context"
	"encoding/json"
	"fmt"
)

const barkDefaultServer = "https://api.day.app"

type Bark struct {
	DeviceKey string
	Endpoint  string
	HC        *HTTPClient
}

func NewBark(server, deviceKey string, hc *HTTPClient) *Bark {
	if server == "" {
		server = barkDefaultServer
	}
	return &Bark{DeviceKey: deviceKey, Endpoint: server, HC: hc}
}

func (b *Bark) Name() string { return "bark" }

func (b *Bark) Send(ctx context.Context, m Message) error {
	payload := map[string]string{"title": Title(m), "body": Body(m)}
	resp, err := b.HC.PostJSONResp(ctx, b.Endpoint+"/"+b.DeviceKey, payload, nil)
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
	if out.Code != 200 {
		return fmt.Errorf("bark 返回 code=%d msg=%s", out.Code, out.Message)
	}
	return nil
}
