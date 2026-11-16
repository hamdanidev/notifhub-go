package notifhub

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const testAPIKey = "test-api-key"

func newServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

func newTestClient(t *testing.T, server *httptest.Server, opts ...Option) *SDK {
	t.Helper()

	client, err := New(server.URL, testAPIKey, opts...)

	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func TestNotifySendsPayloadAndDecodesResponse(t *testing.T) {
	server := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/notify" {
			t.Errorf("path = %q", r.URL.Path)
		}

		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}

		if got := r.Header.Get("X-API-Key"); got != testAPIKey {
			t.Errorf("X-API-Key = %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if payload["idempotency_key"] != "order-123" {
			t.Errorf("idempotency_key = %v", payload["idempotency_key"])
		}

		recipients, ok := payload["recipients"].([]any)
		if !ok || len(recipients) != 2 {
			t.Fatalf("recipients = %#v", payload["recipients"])
		}

		if recipients[0] != "user-1" {
			t.Errorf("recipients[0] = %v", recipients[0])
		}

		override, ok := recipients[1].(map[string]any)
		if !ok || override["id"] != "user-2" || override["email"] != "user2@example.com" {
			t.Errorf("recipients[1] = %#v", recipients[1])
		}

		writeJSON(t, w, http.StatusAccepted, map[string]any{
			"notification_id": "7f34f8a8-6b7d-4f5e-a1f0-1f7b2a3c4d5e",
			"status":          "accepted",
		})
	})

	client := newTestClient(t, server)
	email := "user2@example.com"

	result, err := client.Notify(context.Background(), NotifyRequest{
		IdempotencyKey: "order-123",
		TemplateKey:    "order.confirmed",
		Recipients: []Recipient{
			{ID: "user-1"},
			{ID: "user-2", Email: &email},
		},
		Channels: []string{"inapp", "email"},
		Data:     map[string]any{"order_id": "123"},
	})

	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if result.NotificationID != "7f34f8a8-6b7d-4f5e-a1f0-1f7b2a3c4d5e" {
		t.Errorf("notification id = %q", result.NotificationID)
	}

	if result.Status != "accepted" {
		t.Errorf("status = %q", result.Status)
	}
}

func TestUpsertSubscriber(t *testing.T) {
	server := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/subscribers/user-1" {
			t.Errorf("path = %q", r.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if payload["email"] != "user@example.com" {
			t.Errorf("email = %v", payload["email"])
		}

		writeJSON(t, w, http.StatusOK, map[string]any{
			"id":         "0f7e2a3c-4b5d-4e6f-8a9b-0c1d2e3f4a5b",
			"tenantId":   "default",
			"externalId": "user-1",
			"email":      "user@example.com",
			"phone":      nil,
			"locale":     "id",
			"timezone":   "Asia/Jakarta",
			"metadata":   nil,
			"createdAt":  "2026-01-01T00:00:00Z",
			"updatedAt":  "2026-01-01T00:00:00Z",
		})
	})

	client := newTestClient(t, server)
	email := "user@example.com"
	locale := "id"

	subscriber, err := client.UpsertSubscriber(context.Background(), "user-1", SubscriberInput{
		Email:  &email,
		Locale: &locale,
	})

	if err != nil {
		t.Fatalf("UpsertSubscriber: %v", err)
	}

	if subscriber.ExternalId != "user-1" || subscriber.Timezone != "Asia/Jakarta" {
		t.Errorf("subscriber = %#v", subscriber)
	}
}

func TestTopicPathAndIncludeExpired(t *testing.T) {
	server := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/topics/shift:gudang/members" {
			t.Errorf("path = %q", r.URL.Path)
		}

		if got := r.URL.Query().Get("includeExpired"); got != "true" {
			t.Errorf("includeExpired = %q", got)
		}

		writeJSON(t, w, http.StatusOK, map[string]any{
			"topic": map[string]any{
				"key":         "shift:gudang",
				"mode":        "push",
				"isTimeBound": false,
			},
			"members": []any{
				map[string]any{
					"recipientId": "user-1",
					"addedAt":     "2026-01-01T00:00:00Z",
					"expiresAt":   nil,
				},
			},
			"expiredCount": 0,
		})
	})

	client := newTestClient(t, server)

	members, err := client.GetTopicMembers(context.Background(), "shift:gudang", true)

	if err != nil {
		t.Fatalf("GetTopicMembers: %v", err)
	}

	if members.Topic.Key != "shift:gudang" || len(members.Members) != 1 {
		t.Errorf("members = %#v", members)
	}
}

func TestAPIErrorMapping(t *testing.T) {
	server := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, map[string]any{
			"statusCode": 401,
			"message":    "Invalid API key",
		})
	})

	client := newTestClient(t, server)

	_, err := client.GetSubscriber(context.Background(), "user-1")

	if err == nil {
		t.Fatal("expected error")
	}

	if !IsUnauthorized(err) {
		t.Errorf("IsUnauthorized(%v) = false", err)
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}

	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Message != "Invalid API key" {
		t.Errorf("apiErr = %#v", apiErr)
	}
}

func TestRetryOnlyWithIdempotencyKey(t *testing.T) {
	var calls atomic.Int32

	server := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		attempt := calls.Add(1)

		if attempt == 1 {
			writeJSON(t, w, http.StatusInternalServerError, map[string]any{"message": "temporary"})
			return
		}

		writeJSON(t, w, http.StatusAccepted, map[string]any{
			"notification_id": "7f34f8a8-6b7d-4f5e-a1f0-1f7b2a3c4d5e",
			"status":          "accepted",
		})
	})

	client := newTestClient(t, server, WithRetry(RetryPolicy{
		MaxRetries: 2,
		BaseDelay:  time.Millisecond,
		MaxDelay:   5 * time.Millisecond,
	}))

	_, err := client.Notify(context.Background(), NotifyRequest{
		IdempotencyKey: "retry-key",
		TemplateKey:    "test.notification",
		Recipients:     []Recipient{{ID: "user-1"}},
		Data:           map[string]any{},
	})

	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestNewClientValidation(t *testing.T) {
	if _, err := New("", "key"); err == nil {
		t.Error("expected error for empty base URL")
	}

	if _, err := New("http://localhost:3000", ""); err == nil {
		t.Error("expected error for empty API key")
	}
}

func TestNotifyValidation(t *testing.T) {
	client, err := New("http://localhost:3000", "key")

	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = client.Notify(context.Background(), NotifyRequest{
		TemplateKey: "test.notification",
		Data:        map[string]any{},
	})

	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("error type = %T", err)
	}
}
