package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const telegramTemplate = "https://api.telegram.org/bot%s/sendMessage"

type Telegram struct {
	ChatID   string
	Endpoint string
	HC       *HTTPClient
}

func NewTelegram(botToken, chatID, proxy string, hc *HTTPClient) (*Telegram, error) {
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("代理地址非法 %q", proxy)
		}
		hc = &HTTPClient{
			Base:  &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}},
			sleep: hc.sleep,
		}
	}
	return &Telegram{ChatID: chatID, Endpoint: fmt.Sprintf(telegramTemplate, botToken), HC: hc}, nil
}

func (t *Telegram) Name() string { return "telegram" }

func (t *Telegram) Send(ctx context.Context, m Message) error {
	payload := map[string]string{"chat_id": t.ChatID, "text": Title(m) + "\n" + Body(m)}
	resp, err := t.HC.PostJSONResp(ctx, t.Endpoint, payload, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("响应解析失败: %w", err)
	}
	if !out.Ok {
		return fmt.Errorf("telegram 返回失败: %s", out.Description)
	}
	return nil
}
