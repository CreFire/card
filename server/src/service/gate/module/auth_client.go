package module

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	commonserver "backend/src/common/server"
)

const defaultAuthAddress = "http://127.0.0.1:9002"

type AuthClient interface {
	ConsumeConnectToken(ctx context.Context, token string) (*AuthSession, error)
	ConsumeLoginTicket(ctx context.Context, ticket string) (*AuthSession, error)
	ValidateSession(ctx context.Context, sessionID string) (*AuthSession, error)
}

type HTTPAuthClient struct {
	runtime *commonserver.Runtime
	client  *http.Client
}

type authEnvelope[T any] struct {
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

func (c *HTTPAuthClient) ConsumeLoginTicket(ctx context.Context, ticket string) (*AuthSession, error) {
	var session AuthSession
	if err := c.postJSON(ctx, "/auth/tickets/consume", map[string]string{
		"ticket": ticket,
	}, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *HTTPAuthClient) ConsumeConnectToken(ctx context.Context, token string) (*AuthSession, error) {
	var session AuthSession
	if err := c.postJSON(ctx, "/auth/tokens/consume", map[string]string{
		"token": token,
	}, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *HTTPAuthClient) ValidateSession(ctx context.Context, sessionID string) (*AuthSession, error) {
	var session AuthSession
	if err := c.postJSON(ctx, "/auth/sessions/validate", map[string]string{
		"session_id": sessionID,
	}, &session); err != nil {
		return nil, err
	}
	return &session, nil
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

	var envelope authEnvelope[json.RawMessage]
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
			return ensureHTTP(instances[0].Address), nil
		}
	}
	return defaultAuthAddress, nil
}

func ensureHTTP(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return address
	}
	return "http://" + address
}
