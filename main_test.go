package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestChatAcceptsOpenAICompatibleResponse(t *testing.T) {
	client := NewInfraiClient("test")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ready"}}]}`))}, nil
	})
	var reply map[string]any
	if err := client.request(http.MethodPost, "/chat/completions", chatRequest{Model: "auto"}, &reply); err != nil {
		t.Fatal(err)
	}
	if _, ok := reply["choices"]; !ok {
		t.Fatalf("missing choices: %v", reply)
	}
}

func TestCanSpendCapsCheckout(t *testing.T) {
	tests := []struct {
		name                string
		used, cap, estimate float64
		want                bool
	}{{"within cap", 3, 10, 2, true}, {"exact cap", 8, 10, 2, true}, {"over cap", 9, 10, 2, false}, {"negative estimate", 1, 10, -1, false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canSpend(tt.used, tt.cap, tt.estimate); got != tt.want {
				t.Fatalf("canSpend()=%v,want %v", got, tt.want)
			}
		})
	}
}
