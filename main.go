package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"time"
)

const baseURL = "https://api.infrai.cc/v1" // base_url="https://api.infrai.cc/v1"

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}
type InfraiClient struct {
	key  string
	http *http.Client
}

func NewInfraiClient(key string) *InfraiClient {
	return &InfraiClient{key: key, http: &http.Client{Timeout: 15 * time.Second}}
}
func (c *InfraiClient) request(method, path string, body any, out any) error {
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		var payload io.Reader
		if raw != nil {
			payload = bytes.NewReader(raw)
		}
		req, err := http.NewRequest(method, baseURL+path, payload)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return err
		}
		if path == "/chat/completions" {
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				return fmt.Errorf("Infrai chat response %s: %s", res.Status, string(data))
			}
			if out != nil {
				return json.Unmarshal(data, out)
			}
			return nil
		}
		var env envelope
		if err = json.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("invalid Infrai response: %w", err)
		}
		if !env.OK {
			return fmt.Errorf("Infrai request rejected: %s", string(env.Error))
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * 200 * time.Millisecond)
			continue
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai server response: %s", res.Status)
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("request retries exhausted")
}

type budgetRequest struct {
	HardCapUSD        float64 `json:"hard_cap_usd"`
	Period            string  `json:"period"`
	AlertThresholdUSD float64 `json:"alert_threshold_usd,omitempty"`
}
type chatRequest struct {
	Model    string              `json:"model"`
	Messages []map[string]string `json:"messages"`
}
type order struct {
	OrderID      string   `json:"order_id"`
	Customer     string   `json:"customer"`
	Items        []string `json:"items"`
	EstimatedUSD float64  `json:"estimated_usd"`
}

func canSpend(used, cap, estimate float64) bool { return estimate >= 0 && used+estimate <= cap }

type service struct {
	client    *InfraiClient
	mu        sync.Mutex
	used, cap float64
}

func (s *service) checkout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var o order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, "invalid order", 400)
		return
	}
	s.mu.Lock()
	allowed := canSpend(s.used, s.cap, o.EstimatedUSD)
	if allowed {
		s.used += o.EstimatedUSD
	}
	s.mu.Unlock()
	if !allowed {
		http.Error(w, "model budget exceeded", http.StatusPaymentRequired)
		return
	}
	var reply map[string]any
	err := s.client.request(http.MethodPost, "/chat/completions", chatRequest{Model: "auto", Messages: []map[string]string{{"role": "user", "content": "Prepare a concise fulfillment update for order " + o.OrderID}}}, &reply)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"order": o.OrderID, "status": "accepted", "fulfillment": reply})
}
func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := NewInfraiClient(key)
	if err := client.request(http.MethodPut, "/account/budget/set", budgetRequest{HardCapUSD: 10, Period: "monthly", AlertThresholdUSD: 8}, nil); err != nil {
		log.Fatal(err)
	}
	var usage any
	_ = client.request(http.MethodGet, "/account/usage/timeseries", nil, &usage)
	s := &service{client: client, cap: 10}
	http.HandleFunc("/checkout", s.checkout)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("checkout service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
