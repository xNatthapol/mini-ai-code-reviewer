// Package api exposes the review contract over HTTP.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/xNatthapol/mini-ai-code-reviewer/backend/internal/review"
)

// The JSON envelope may be larger than the source because of JSON escaping.
const maxRequestBytes = 128 * 1024

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// NewHandler processes lint requests; the caller registers its route.
func NewHandler(reviewer review.Reviewer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := http.MaxBytesReader(w, r.Body, maxRequestBytes)
		defer body.Close()
		decoder := json.NewDecoder(body)
		decoder.DisallowUnknownFields()

		var req review.Request
		err := decoder.Decode(&req)
		if err == nil {
			// Require EOF after the object; trailing whitespace is allowed.
			err = decoder.Decode(new(any))
			switch err {
			case io.EOF:
				err = nil
			case nil:
				err = errors.New("multiple JSON values")
			}
		}
		if err != nil {
			var sizeError *http.MaxBytesError
			if errors.As(err, &sizeError) {
				writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body exceeds 128 KiB.")
			} else {
				writeError(w, http.StatusBadRequest, "invalid_request", "Provide exactly one JSON object containing code and language.")
			}
			return
		}
		req.Code = review.NormalizeCode(req.Code)
		if err := review.ValidateRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		result, err := reviewer.Review(r.Context(), req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				writeError(w, http.StatusGatewayTimeout, "review_timeout", "The review timed out. Please try again.")
			} else {
				writeError(w, http.StatusBadGateway, "review_failed", "The review could not be completed. Please try again.")
			}
			return
		}
		if err := review.ValidateResult(result, strings.Count(req.Code, "\n")+1); err != nil {
			writeError(w, http.StatusBadGateway, "invalid_review", "The reviewer returned an invalid response. Please try again.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var response errorResponse
	response.Error.Code = code
	response.Error.Message = message
	writeJSON(w, status, response)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Headers are already sent; a write failure cannot become another response.
	_ = json.NewEncoder(w).Encode(body)
}
