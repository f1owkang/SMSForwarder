package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	return code == 500 || code == 502 || code == 504
}

func (h *HTTPClient) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			h.sleep(backoff << (attempt - 1))
		}
		rctx, cancel := context.WithTimeout(ctx, reqTimeout)
		resp, err := h.Base.Do(req.WithContext(rctx))
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if retryable(resp.StatusCode) {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("上游返回可重试状态码 %d", resp.StatusCode)
			continue
		}
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
