package orderhttp

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"order-tracking/internal/order/application"
	"order-tracking/internal/order/infra/memory"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := application.NewService(memory.NewRepository(), log)

	mux := http.NewServeMux()
	NewHandler(svc, log).RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, method, url, body string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil && err != io.EOF {
		t.Fatalf("decode response: %v", err)
	}
	return resp, payload
}

const validBody = `{"customer_id":"customer-1","items":[{"sku":"KB-001","quantity":2,"unit_price_cents":15000}]}`

func TestCreateOrder(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "created", body: validBody, wantStatus: http.StatusCreated},
		{name: "malformed json", body: `{"customer_id":`, wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "unknown field", body: `{"customer":"x"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "validation error", body: `{"customer_id":"c","items":[]}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, payload := do(t, http.MethodPost, srv.URL+"/orders", tt.body)
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %v)", resp.StatusCode, tt.wantStatus, payload)
			}
			if tt.wantCode != "" {
				errObj, _ := payload["error"].(map[string]any)
				if errObj["code"] != tt.wantCode {
					t.Fatalf("error code = %v, want %s", errObj["code"], tt.wantCode)
				}
				return
			}
			if payload["status"] != "CREATED" || payload["total_cents"] != float64(30000) {
				t.Fatalf("unexpected body: %v", payload)
			}
			if !strings.HasPrefix(resp.Header.Get("Location"), "/orders/") {
				t.Fatalf("missing Location header")
			}
		})
	}
}

func TestOrderLifecycle(t *testing.T) {
	srv := newTestServer(t)

	_, created := do(t, http.MethodPost, srv.URL+"/orders", validBody)
	id := created["id"].(string)

	resp, body := do(t, http.MethodPatch, srv.URL+"/orders/"+id+"/status", `{"status":"PAID"}`)
	if resp.StatusCode != http.StatusOK || body["status"] != "PAID" {
		t.Fatalf("pay: status=%d body=%v", resp.StatusCode, body)
	}
	if next := body["next_statuses"].([]any); len(next) != 2 || next[0] != "PICKING" {
		t.Fatalf("next_statuses = %v, want [PICKING CANCELED]", next)
	}

	resp, _ = do(t, http.MethodPatch, srv.URL+"/orders/"+id+"/status", `{"status":"DELIVERED"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("invalid transition: status=%d, want 409", resp.StatusCode)
	}

	resp, body = do(t, http.MethodGet, srv.URL+"/orders/"+id+"/history", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("history: status=%d", resp.StatusCode)
	}
	if entries := body["data"].([]any); len(entries) != 2 {
		t.Fatalf("history entries = %d, want 2", len(entries))
	}

	resp, body = do(t, http.MethodGet, srv.URL+"/orders?status=PAID", "")
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("list: status=%d body=%v", resp.StatusCode, body)
	}
}

func TestGetOrder_Errors(t *testing.T) {
	srv := newTestServer(t)

	resp, _ := do(t, http.MethodGet, srv.URL+"/orders/not-a-uuid", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid id: status=%d, want 400", resp.StatusCode)
	}

	resp, _ = do(t, http.MethodGet, srv.URL+"/orders/"+uuid.NewString(), "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing order: status=%d, want 404", resp.StatusCode)
	}

	resp, _ = do(t, http.MethodGet, srv.URL+"/orders?limit=abc", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid limit: status=%d, want 400", resp.StatusCode)
	}
}
