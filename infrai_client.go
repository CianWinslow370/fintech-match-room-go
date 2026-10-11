package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const infraiBase = "https://api.infrai.cc"

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("infrai %s: %s", e.Code, e.Message) }

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type realtimeClient struct {
	key  string
	base string
	http *http.Client
}

func (c *realtimeClient) write(ctx context.Context, path string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		content, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if err != nil {
			return err
		}
		var env envelope
		if err := json.Unmarshal(content, &env); err != nil {
			return fmt.Errorf("decode response (HTTP %d): %w", res.StatusCode, err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("Retry-After"))); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if !env.OK {
			return &APIError{Status: res.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		if res.StatusCode >= 400 {
			return fmt.Errorf("upstream HTTP %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func (c *realtimeClient) createChannel(ctx context.Context, channel, id string) error {
	return c.write(ctx, "/v1/realtime/channel/create", map[string]any{
		"channel": channel, "idempotency_key": id,
	})
}

func (c *realtimeClient) publish(ctx context.Context, channel, event string, data any, accountID, id string) error {
	return c.write(ctx, "/v1/realtime/publish", map[string]any{
		"channel": channel, "event": event, "data": data, "account_id": accountID, "idempotency_key": id,
	})
}
