package channel

import (
	"context"
	"fmt"
)

type Webhook struct {
	Endpoint string
	HC       *HTTPClient
}

func NewWebhook(endpoint string, hc *HTTPClient) *Webhook {
	return &Webhook{Endpoint: endpoint, HC: hc}
}

func (w *Webhook) Name() string { return "webhook" }

func (w *Webhook) Send(ctx context.Context, m Message) error {
	resp, err := w.HC.PostJSONResp(ctx, w.Endpoint, map[string]string{
		"number": m.Number, "text": m.Text, "keyword": m.Keyword, "timestamp": m.Timestamp,
	}, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("webhook 返回状态码 %d", resp.StatusCode)
	}
	return nil
}
