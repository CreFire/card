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

const defaultGateAddress = "http://127.0.0.1:9004"

type GateClient interface {
	OpenConnection(ctx context.Context, input OpenConnectionInput) (*GateGamer, error)
	LoginByToken(ctx context.Context, input GateLoginByTokenInput) (*GateGamer, error)
}

type HTTPGateClient struct {
	runtime *commonserver.Runtime
	client  *http.Client
}

func NewHTTPGateClient(runtime *commonserver.Runtime) *HTTPGateClient {
	return &HTTPGateClient{
		runtime: runtime,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *HTTPGateClient) OpenConnection(ctx context.Context, input OpenConnectionInput) (*GateGamer, error) {
	var gamer GateGamer
	if err := c.postJSON(ctx, "/gate/connections/open", input, &gamer); err != nil {
		return nil, err
	}
	return &gamer, nil
}

func (c *HTTPGateClient) LoginByToken(ctx context.Context, input GateLoginByTokenInput) (*GateGamer, error) {
	var gamer GateGamer
	if err := c.postJSON(ctx, "/gate/login/token", input, &gamer); err != nil {
		return nil, err
	}
	return &gamer, nil
}

func (c *HTTPGateClient) postJSON(ctx context.Context, path string, requestBody any, out any) error {
	baseURL, err := c.baseURL(ctx)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal gate request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create gate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("call gate api %s: %w", path, err)
	}
	defer resp.Body.Close()

	var envelope apiEnvelope[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode gate response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest || envelope.Code != "ok" {
		if envelope.Message == "" {
			envelope.Message = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("gate api %s failed: %s", path, envelope.Message)
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode gate payload: %w", err)
	}
	return nil
}

func (c *HTTPGateClient) baseURL(ctx context.Context) (string, error) {
	if c.runtime != nil && c.runtime.Registry != nil {
		instances, err := c.runtime.Registry.Discover(ctx, "gate")
		if err == nil && len(instances) > 0 && instances[0].Address != "" {
			return ensureHTTPPrefix(instances[0].Address), nil
		}
	}
	return defaultGateAddress, nil
}
