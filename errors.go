package notifhub

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// RetryPolicy controls retries for idempotent operations.
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryPolicy is a conservative policy: three retries with exponential backoff.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  250 * time.Millisecond,
		MaxDelay:   2 * time.Second,
	}
}

type options struct {
	httpClient HttpRequestDoer
	userAgent  string
	retry      RetryPolicy
}

// Option configures the SDK client.
type Option func(*options) error

// WithHTTPDoer replaces the default HTTP client (which has a 30s timeout).
func WithHTTPDoer(client HttpRequestDoer) Option {
	return func(o *options) error {
		if client == nil {
			return errors.New("notifhub: http client must not be nil")
		}
		o.httpClient = client
		return nil
	}
}

// WithUserAgent sets the User-Agent header. The default is "notifhub-go/0.8.0".
func WithUserAgent(userAgent string) Option {
	return func(o *options) error {
		o.userAgent = userAgent
		return nil
	}
}

// WithRetry enables retries. Without this option the client never retries.
func WithRetry(policy RetryPolicy) Option {
	return func(o *options) error {
		if policy.MaxRetries < 0 {
			return errors.New("notifhub: MaxRetries must be >= 0")
		}
		if policy.BaseDelay <= 0 || policy.MaxDelay <= 0 {
			return errors.New("notifhub: retry delays must be positive")
		}
		o.retry = policy
		return nil
	}
}

func (c *SDK) do(ctx context.Context, retryable bool, call func() (int, error)) error {
	policy := c.retry

	var lastErr error

	for attempt := 0; ; attempt++ {
		status, err := call()

		if err == nil {
			if status < http.StatusBadRequest ||
				!retryable ||
				!isRetryableStatus(status) ||
				attempt >= policy.MaxRetries {
				return nil
			}
		} else {
			lastErr = err

			if !retryable || attempt >= policy.MaxRetries {
				return err
			}
		}

		delay := backoffDelay(policy, attempt)

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func backoffDelay(policy RetryPolicy, attempt int) time.Duration {
	if policy.BaseDelay <= 0 {
		return 0
	}

	shift := attempt
	if shift > 5 {
		shift = 5
	}

	delay := policy.BaseDelay * time.Duration(1<<shift)

	if policy.MaxDelay > 0 && delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	jitter := time.Duration(rand.Int63n(int64(policy.BaseDelay/2) + 1))

	return delay + jitter
}

// APIError is returned for non-2xx responses from NotifHub.
type APIError struct {
	StatusCode int
	Message    string
	Body       []byte
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("notifhub: %s (status %d)", e.Message, e.StatusCode)
	}

	return fmt.Sprintf("notifhub: request failed with status %d", e.StatusCode)
}

// IsNotFound reports whether err is a 404 API error.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// IsUnauthorized reports whether err is a 401 API error.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}

// IsRateLimited reports whether err is a 429 API error.
func IsRateLimited(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests
}

// IsValidation reports whether err is a 400 or 422 API error.
func IsValidation(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) &&
		(apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusUnprocessableEntity)
}

// ValidationError is returned for locally invalid SDK input.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return "notifhub: " + e.Message
	}
	return fmt.Sprintf("notifhub: invalid %s: %s", e.Field, e.Message)
}

func newAPIError(response *http.Response, body []byte) *APIError {
	apiErr := &APIError{Body: body}

	if response != nil {
		apiErr.StatusCode = response.StatusCode
	}

	message := strings.TrimSpace(string(body))
	apiErr.Message = message

	var payload struct {
		Message any `json:"message"`
	}

	if err := jsonUnmarshal(body, &payload); err == nil {
		switch value := payload.Message.(type) {
		case string:
			if value != "" {
				apiErr.Message = value
			}
		case []any:
			parts := make([]string, 0, len(value))
			for _, item := range value {
				if text, ok := item.(string); ok && text != "" {
					parts = append(parts, text)
				}
			}
			if len(parts) > 0 {
				apiErr.Message = strings.Join(parts, "; ")
			}
		}
	}

	return apiErr
}
