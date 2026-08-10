package monitorprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	TypeAIModel          = "ai_model"
	TypeMinecraftJava    = "minecraft_java"
	TypeMinecraftBedrock = "minecraft_bedrock"

	AIFormatAnthropicMessages = "anthropic_messages"
	AIFormatChatCompletions   = "chat_completions"
	AIFormatResponses         = "responses"

	StatusUp      = "up"
	StatusSlow    = "slow"
	StatusDown    = "down"
	StatusUnknown = "unknown"
)

type Request struct {
	Type     string
	Target   string
	Port     int
	Timeout  time.Duration
	AIFormat string
	AIModel  string
	AIAPIKey string
}

type Result struct {
	Status       string
	ResponseTime int
	ErrorCode    string
	Error        error
	Metadata     map[string]any
}

func Check(ctx context.Context, request Request) Result {
	startedAt := time.Now()
	result := check(ctx, request)
	result.ResponseTime = int(time.Since(startedAt).Milliseconds())
	return result
}

func check(ctx context.Context, request Request) Result {
	switch request.Type {
	case TypeAIModel:
		return checkAI(ctx, request)
	case TypeMinecraftJava:
		return checkMinecraftJava(ctx, request)
	case TypeMinecraftBedrock:
		return checkMinecraftBedrock(ctx, request)
	default:
		return failed("unsupported_type", fmt.Errorf("不支持的监测类型或 AI 接口格式"))
	}
}

func checkAI(ctx context.Context, request Request) Result {
	payload, err := buildAIRequestPayload(request)
	if err != nil {
		return failed("invalid_request", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return failed("invalid_request", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, request.Target, bytes.NewReader(body))
	if err != nil {
		return failed("invalid_request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if request.AIAPIKey != "" {
		if request.AIFormat == AIFormatAnthropicMessages {
			req.Header.Set("x-api-key", request.AIAPIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+request.AIAPIKey)
		}
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return networkFailed(err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return failed("invalid_response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return failed(classifyAIHTTPStatus(resp.StatusCode), fmt.Errorf("HTTP %d", resp.StatusCode))
	}
	metadata, err := validateAIResponse(request, responseBody)
	if err != nil {
		return failed("invalid_response", err)
	}
	return Result{Status: StatusUp, Metadata: metadata}
}

func classifyAIHTTPStatus(statusCode int) string {
	switch {
	case statusCode >= 300 && statusCode < 400:
		return "redirect_rejected"
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return "authentication_error"
	case statusCode == http.StatusNotFound:
		return "model_not_found"
	case statusCode == http.StatusTooManyRequests:
		return "rate_limited"
	case statusCode >= 500:
		return "provider_error"
	default:
		return "http_error"
	}
}

func buildAIRequestPayload(request Request) (map[string]any, error) {
	switch request.AIFormat {
	case AIFormatAnthropicMessages:
		return map[string]any{
			"model":      request.AIModel,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
			"max_tokens": 1,
		}, nil
	case AIFormatChatCompletions:
		return map[string]any{
			"model":      request.AIModel,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
			"max_tokens": 1,
			"stream":     false,
		}, nil
	case AIFormatResponses:
		return map[string]any{
			"model":             request.AIModel,
			"input":             "ping",
			"max_output_tokens": 1,
			"stream":            false,
		}, nil
	default:
		return nil, fmt.Errorf("不支持的 AI 接口格式: %s", request.AIFormat)
	}
}

func validateAIResponse(request Request, body []byte) (map[string]any, error) {
	var payload struct {
		ID                string `json:"id"`
		Object            string `json:"object"`
		Type              string `json:"type"`
		Model             string `json:"model"`
		Choices           []any  `json:"choices"`
		Content           []any  `json:"content"`
		Status            string `json:"status"`
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("AI 响应不是合法 JSON")
	}
	valid := false
	switch request.AIFormat {
	case AIFormatAnthropicMessages:
		valid = payload.ID != "" && payload.Type == "message" && payload.Content != nil
	case AIFormatChatCompletions:
		valid = payload.ID != "" && len(payload.Choices) > 0
	case AIFormatResponses:
		valid = payload.ID != "" && payload.Object == "response" && (payload.Status == "completed" ||
			(payload.Status == "incomplete" && payload.IncompleteDetails.Reason == "max_output_tokens"))
	}
	if !valid {
		return nil, fmt.Errorf("AI 响应结构无效")
	}
	return map[string]any{
		"kind":             "ai",
		"api_format":       request.AIFormat,
		"configured_model": request.AIModel,
		"response_model":   payload.Model,
	}, nil
}

func failed(code string, err error) Result {
	return Result{Status: StatusDown, ErrorCode: code, Error: err}
}

func networkFailed(err error) Result {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return Result{Status: StatusSlow, ErrorCode: "timeout", Error: err}
	}
	return failed("network_error", err)
}
