// Package groq implements the reviewer using Groq's chat completions API.
package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xNatthapol/mini-ai-code-reviewer/backend/internal/review"
)

const (
	endpoint         = "https://api.groq.com/openai/v1/chat/completions"
	reviewTimeout    = 30 * time.Second
	maxResponseBytes = 256 * 1024
)

type Client struct {
	httpClient *http.Client
	apiKey     string
	model      string
	endpoint   string
}

func NewClient(httpClient *http.Client, apiKey, model string) *Client {
	return &Client{httpClient: httpClient, apiKey: apiKey, model: model, endpoint: endpoint}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type completionRequest struct {
	Model               string    `json:"model"`
	Messages            []message `json:"messages"`
	MaxCompletionTokens int       `json:"max_completion_tokens"`
	ResponseFormat      struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name   string          `json:"name"`
			Strict bool            `json:"strict"`
			Schema json.RawMessage `json:"schema"`
		} `json:"json_schema"`
	} `json:"response_format"`
}

type completionResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role    string  `json:"role"`
			Content string  `json:"content"`
			Refusal *string `json:"refusal"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) Review(ctx context.Context, req review.Request) (review.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reviewTimeout)
	defer cancel()
	req.Code = review.NormalizeCode(req.Code)

	payload := completionRequest{
		Model:               c.model,
		Messages:            []message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: sourceMessage(req)}},
		MaxCompletionTokens: 4096,
	}
	payload.ResponseFormat.Type = "json_schema"
	payload.ResponseFormat.JSONSchema.Name = "code_review"
	payload.ResponseFormat.JSONSchema.Strict = true
	payload.ResponseFormat.JSONSchema.Schema = json.RawMessage(resultSchema)
	data, err := json.Marshal(payload)
	if err != nil {
		return review.Result{}, fmt.Errorf("encode review request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(data))
	if err != nil {
		return review.Result{}, fmt.Errorf("create review request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return review.Result{}, fmt.Errorf("request review: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return review.Result{}, fmt.Errorf("Groq returned HTTP %d", response.StatusCode)
	}

	// Read one extra byte to distinguish an oversized response from a complete one.
	data, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return review.Result{}, fmt.Errorf("read review response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return review.Result{}, errors.New("review response exceeds size limit")
	}
	var completion completionResponse
	if err := json.Unmarshal(data, &completion); err != nil {
		return review.Result{}, fmt.Errorf("decode review response: %w", err)
	}
	if len(completion.Choices) != 1 {
		return review.Result{}, errors.New("expected one review choice")
	}
	choice := completion.Choices[0]
	if choice.FinishReason != "stop" {
		return review.Result{}, errors.New("review did not complete normally")
	}
	if choice.Message.Refusal != nil {
		return review.Result{}, errors.New("review was refused")
	}
	if choice.Message.Role != "assistant" {
		return review.Result{}, errors.New("unexpected review message role")
	}

	var result review.Result
	decoder := json.NewDecoder(strings.NewReader(choice.Message.Content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return review.Result{}, fmt.Errorf("decode review findings: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return review.Result{}, errors.New("unexpected data after review findings")
	}
	if err := review.ValidateResult(result, strings.Count(req.Code, "\n")+1); err != nil {
		return review.Result{}, fmt.Errorf("invalid review findings: %w", err)
	}
	return result, nil
}
