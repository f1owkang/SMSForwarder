package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	reqTimeout = 5 * time.Second
	maxRetries = 2
	backoff    = time.Second
)

type HTTPClient struct {
	Base  *http.Client
	sleep func(time.Duration)
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{Base: &http.Client{}, sleep: time.Sleep}
}

func retryable(code int) bool {
	return code == 500 || code == 502 || code == 503 || code == 504 || code == 429
}

// retryDelay 计算下一次重试前的等待：优先遵循上游 Retry-After（秒数），
// 否则按指数退避 1s、2s。
func retryDelay(resp *http.Response, attempt int) time.Duration {
	wait := backoff << attempt
	if after := resp.Header.Get("Retry-After"); after != "" {
		if secs, err := strconv.Atoi(after); err == nil && secs >= 0 {
			wait = time.Duration(secs) * time.Second
		}
	}
	return wait
}

type bodyWithCancel struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *bodyWithCancel) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func (h *HTTPClient) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		rctx, cancel := context.WithTimeout(ctx, reqTimeout)
		resp, err := h.Base.Do(req.WithContext(rctx))
		if err != nil {
			cancel()
			lastErr = err
			if attempt < maxRetries {
				h.sleep(backoff << attempt)
			}
			continue
		}
		if retryable(resp.StatusCode) {
			wait := retryDelay(resp, attempt)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("上游返回可重试状态码 %d", resp.StatusCode)
			if attempt < maxRetries {
				h.sleep(wait)
			}
			continue
		}
		resp.Body = &bodyWithCancel{ReadCloser: resp.Body, cancel: cancel}
		return resp, nil
	}
	return nil, lastErr
}

func (h *HTTPClient) PostFormResp(ctx context.Context, rawURL string, form url.Values) (*http.Response, error) {
	var body io.Reader
	if form != nil {
		body = bytes.NewReader([]byte(form.Encode()))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return h.do(ctx, req)
}

func (h *HTTPClient) PostJSONResp(ctx context.Context, rawURL string, payload any, extra http.Header) (*http.Response, error) {
	buf := &bytes.Buffer{}
	if payload != nil {
		if err := json.NewEncoder(buf).Encode(payload); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	return h.do(ctx, req)
}
