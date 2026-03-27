package module

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	commonserver "backend/src/common/server"
)

const defaultAuthAddress = "http://127.0.0.1:9002"

type AuthClient interface {
	Login(ctx context.Context, input AuthLoginInput) (*AuthLoginResult, error)
}

type HTTPAuthClient struct {
	runtime *commonserver.Runtime
	client  *http.Client
}

type apiEnvelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func NewHTTPAuthClient(runtime *commonserver.Runtime) *HTTPAuthClient {
	return &HTTPAuthClient{
		runtime: runtime,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *HTTPAuthClient) Login(ctx context.Context, input AuthLoginInput) (*AuthLoginResult, error) {
	var result AuthLoginResult
	if err := c.postJSON(ctx, "/auth/login", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *HTTPAuthClient) postJSON(ctx context.Context, path string, requestBody any, out any) error {
	baseURL, err := c.baseURL(ctx)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("call auth api %s: %w", path, err)
	}
	defer resp.Body.Close()

	var envelope apiEnvelope[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode auth response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest || envelope.Code != "ok" {
		if envelope.Message == "" {
			envelope.Message = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("auth api %s failed: %s", path, envelope.Message)
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode auth payload: %w", err)
	}
	return nil
}

func (c *HTTPAuthClient) baseURL(ctx context.Context) (string, error) {
	if c.runtime != nil && c.runtime.Registry != nil {
		instances, err := c.runtime.Registry.Discover(ctx, "auth")
		if err == nil && len(instances) > 0 && instances[0].Address != "" {
			return ensureHTTPPrefix(instances[0].Address), nil
		}
	}
	return defaultAuthAddress, nil
}
