package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func getRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

type meta struct {
	RequestID string `json:"request_id"`
}

type listMeta struct {
	RequestID string `json:"request_id"`
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
	Total     int    `json:"total"`
}

type envelope struct {
	Data any  `json:"data"`
	Meta meta `json:"meta"`
}

type listEnvelope struct {
	Data any      `json:"data"`
	Meta listMeta `json:"meta"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
	Meta  meta     `json:"meta"`
}

func RespondJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(envelope{
		Data: data,
		Meta: meta{RequestID: getRequestID(r.Context())},
	})
}

func RespondList(w http.ResponseWriter, r *http.Request, status int, data any, page, perPage, total int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(listEnvelope{
		Data: data,
		Meta: listMeta{RequestID: getRequestID(r.Context()), Page: page, PerPage: perPage, Total: total},
	})
}

func RespondError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorEnvelope{
		Error: apiError{Code: code, Message: message},
		Meta:  meta{RequestID: getRequestID(r.Context())},
	})
}

func ParsePagination(r *http.Request) (page, perPage int) {
	page = 1
	perPage = 25
	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if pp, err := strconv.Atoi(v); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}
	return
}

func DecodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}
