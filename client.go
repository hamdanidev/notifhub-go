package notifhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

const sdkVersion = "0.8.0"

// SDK is the NotifHub Go SDK client.
type SDK struct {
	api    *ClientWithResponses
	apiKey string
	retry  RetryPolicy
}

// NewClient creates a client for the NotifHub public API.
//
// baseURL is the service root without /api/v1 (for example "http://localhost:3000").
func New(baseURL, apiKey string, opts ...Option) (*SDK, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, &ValidationError{Field: "baseURL", Message: "must not be empty"}
	}

	if strings.TrimSpace(apiKey) == "" {
		return nil, &ValidationError{Field: "apiKey", Message: "must not be empty"}
	}

	config := options{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "notifhub-go/" + sdkVersion,
	}

	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return nil, err
		}
	}

	generatedOptions := []ClientOption{
		WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			if config.userAgent != "" {
				req.Header.Set("User-Agent", config.userAgent)
			}
			return nil
		}),
	}

	if config.httpClient != nil {
		generatedOptions = append(generatedOptions, WithHTTPClient(config.httpClient))
	}

	api, err := NewClientWithResponses(strings.TrimRight(baseURL, "/"), generatedOptions...)
	if err != nil {
		return nil, fmt.Errorf("notifhub: create client: %w", err)
	}

	return &SDK{api: api, apiKey: apiKey, retry: config.retry}, nil
}

// Notify sends a notification to recipients and/or topics.
//
// The call is retried (when WithRetry is enabled) only if IdempotencyKey is set,
// because the server deduplicates repeated requests by that key.
func (c *SDK) Notify(ctx context.Context, request NotifyRequest) (*NotifyResult, error) {
	if err := validateNotify(request); err != nil {
		return nil, err
	}

	payload := map[string]any{
		"idempotency_key": request.IdempotencyKey,
		"template_key":    request.TemplateKey,
		"data":            request.Data,
	}

	if len(request.Recipients) > 0 {
		recipients := make([]any, 0, len(request.Recipients))

		for _, recipient := range request.Recipients {
			recipients = append(recipients, recipient.raw())
		}

		payload["recipients"] = recipients
	}

	if len(request.Topics) > 0 {
		payload["topics"] = request.Topics
	}

	if len(request.Channels) > 0 {
		payload["channels"] = request.Channels
	}

	if request.Priority != "" {
		payload["priority"] = request.Priority
	}

	if request.TenantID != "" {
		payload["tenant_id"] = request.TenantID
	}

	if request.ScheduledAt != nil {
		payload["scheduled_at"] = request.ScheduledAt.UTC().Format(time.RFC3339)
	}

	body, err := json.Marshal(payload)

	if err != nil {
		return nil, fmt.Errorf("notifhub: encode notify request: %w", err)
	}

	params := &IngestControllerNotifyParams{XAPIKey: c.apiKey}

	var response *IngestControllerNotifyResponse

	err = c.do(ctx, request.IdempotencyKey != "", func() (int, error) {
		result, callErr := c.api.IngestControllerNotifyWithBodyWithResponse(
			ctx,
			params,
			"application/json",
			bytes.NewReader(body),
		)

		if callErr != nil {
			return 0, callErr
		}

		response = result

		return result.StatusCode(), nil
	})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("notifhub: notify returned no response")
	}

	if response.StatusCode() >= http.StatusBadRequest {
		return nil, newAPIError(response.HTTPResponse, response.Body)
	}

	if response.JSON202 == nil {
		return nil, fmt.Errorf(
			"notifhub: unexpected notify response status %d",
			response.StatusCode(),
		)
	}

	return &NotifyResult{
		NotificationID: response.JSON202.NotificationId.String(),
		Status:         string(response.JSON202.Status),
	}, nil
}

// UpsertSubscriber creates or updates a subscriber by external ID.
func (c *SDK) UpsertSubscriber(
	ctx context.Context,
	externalID string,
	input SubscriberInput,
) (*Subscriber, error) {
	if strings.TrimSpace(externalID) == "" {
		return nil, &ValidationError{Field: "externalID", Message: "must not be empty"}
	}

	dto := UpsertSubscriberDto{}

	if input.Email != nil {
		email := openapi_types.Email(*input.Email)
		dto.Email = &email
	}

	if input.Phone != nil {
		dto.Phone = input.Phone
	}

	if input.Locale != nil {
		dto.Locale = input.Locale
	}

	if input.Timezone != nil {
		dto.Timezone = input.Timezone
	}

	if input.Metadata != nil {
		metadata := input.Metadata
		dto.Metadata = &metadata
	}

	params := &SubscribersControllerUpsertParams{XAPIKey: c.apiKey}

	var response *SubscribersControllerUpsertResponse

	err := c.do(ctx, true, func() (int, error) {
		result, callErr := c.api.SubscribersControllerUpsertWithResponse(
			ctx,
			externalID,
			params,
			dto,
		)

		if callErr != nil {
			return 0, callErr
		}

		response = result

		return result.StatusCode(), nil
	})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("notifhub: upsert subscriber returned no response")
	}

	if response.StatusCode() >= http.StatusBadRequest {
		return nil, newAPIError(response.HTTPResponse, response.Body)
	}

	if response.JSON200 == nil {
		return nil, fmt.Errorf(
			"notifhub: unexpected upsert subscriber status %d",
			response.StatusCode(),
		)
	}

	return response.JSON200, nil
}

// GetSubscriber fetches a subscriber by external ID.
func (c *SDK) GetSubscriber(ctx context.Context, externalID string) (*Subscriber, error) {
	if strings.TrimSpace(externalID) == "" {
		return nil, &ValidationError{Field: "externalID", Message: "must not be empty"}
	}

	params := &SubscribersControllerGetParams{XAPIKey: c.apiKey}

	var response *SubscribersControllerGetResponse

	err := c.do(ctx, true, func() (int, error) {
		result, callErr := c.api.SubscribersControllerGetWithResponse(ctx, externalID, params)

		if callErr != nil {
			return 0, callErr
		}

		response = result

		return result.StatusCode(), nil
	})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("notifhub: get subscriber returned no response")
	}

	if response.StatusCode() >= http.StatusBadRequest {
		return nil, newAPIError(response.HTTPResponse, response.Body)
	}

	if response.JSON200 == nil {
		return nil, fmt.Errorf(
			"notifhub: unexpected get subscriber status %d",
			response.StatusCode(),
		)
	}

	return response.JSON200, nil
}

// ReplaceTopicMembers replaces the full membership of a topic (creates it if missing).
func (c *SDK) ReplaceTopicMembers(
	ctx context.Context,
	key string,
	input TopicMembersInput,
) (*TopicMutation, error) {
	if strings.TrimSpace(key) == "" {
		return nil, &ValidationError{Field: "key", Message: "must not be empty"}
	}

	dto, err := input.toDTO()
	if err != nil {
		return nil, err
	}

	params := &TopicsControllerReplaceMembersParams{XAPIKey: c.apiKey}

	var response *TopicsControllerReplaceMembersResponse

	err = c.do(ctx, true, func() (int, error) {
		result, callErr := c.api.TopicsControllerReplaceMembersWithResponse(
			ctx,
			key,
			params,
			dto,
		)

		if callErr != nil {
			return 0, callErr
		}

		response = result

		return result.StatusCode(), nil
	})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("notifhub: replace topic members returned no response")
	}

	if response.StatusCode() >= http.StatusBadRequest {
		return nil, newAPIError(response.HTTPResponse, response.Body)
	}

	if response.JSON200 == nil {
		return nil, fmt.Errorf(
			"notifhub: unexpected replace topic members status %d",
			response.StatusCode(),
		)
	}

	return response.JSON200, nil
}

// AddTopicMembers adds or updates topic members.
func (c *SDK) AddTopicMembers(
	ctx context.Context,
	key string,
	input TopicMembersInput,
) (*TopicMutation, error) {
	if strings.TrimSpace(key) == "" {
		return nil, &ValidationError{Field: "key", Message: "must not be empty"}
	}

	dto, err := input.toDTO()
	if err != nil {
		return nil, err
	}

	params := &TopicsControllerAddMembersParams{XAPIKey: c.apiKey}

	var response *TopicsControllerAddMembersResponse

	err = c.do(ctx, true, func() (int, error) {
		result, callErr := c.api.TopicsControllerAddMembersWithResponse(ctx, key, params, dto)

		if callErr != nil {
			return 0, callErr
		}

		response = result

		return result.StatusCode(), nil
	})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("notifhub: add topic members returned no response")
	}

	if response.StatusCode() >= http.StatusBadRequest {
		return nil, newAPIError(response.HTTPResponse, response.Body)
	}

	if response.JSON200 == nil {
		return nil, fmt.Errorf(
			"notifhub: unexpected add topic members status %d",
			response.StatusCode(),
		)
	}

	return response.JSON200, nil
}

// GetTopicMembers lists topic members, optionally including expired ones.
func (c *SDK) GetTopicMembers(
	ctx context.Context,
	key string,
	includeExpired bool,
) (*TopicMembers, error) {
	if strings.TrimSpace(key) == "" {
		return nil, &ValidationError{Field: "key", Message: "must not be empty"}
	}

	params := &TopicsControllerGetMembersParams{
		XAPIKey:        c.apiKey,
		IncludeExpired: &includeExpired,
	}

	var response *TopicsControllerGetMembersResponse

	err := c.do(ctx, true, func() (int, error) {
		result, callErr := c.api.TopicsControllerGetMembersWithResponse(ctx, key, params)

		if callErr != nil {
			return 0, callErr
		}

		response = result

		return result.StatusCode(), nil
	})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("notifhub: get topic members returned no response")
	}

	if response.StatusCode() >= http.StatusBadRequest {
		return nil, newAPIError(response.HTTPResponse, response.Body)
	}

	if response.JSON200 == nil {
		return nil, fmt.Errorf(
			"notifhub: unexpected get topic members status %d",
			response.StatusCode(),
		)
	}

	return response.JSON200, nil
}

func (r Recipient) raw() any {
	if r.Email == nil && r.Phone == nil && r.Locale == nil && r.Timezone == nil && len(r.Metadata) == 0 {
		return r.ID
	}

	value := map[string]any{"id": r.ID}

	if r.Email != nil {
		value["email"] = *r.Email
	}

	if r.Phone != nil {
		value["phone"] = *r.Phone
	}

	if r.Locale != nil {
		value["locale"] = *r.Locale
	}

	if r.Timezone != nil {
		value["timezone"] = *r.Timezone
	}

	if len(r.Metadata) > 0 {
		value["metadata"] = r.Metadata
	}

	return value
}

func (input TopicMembersInput) toDTO() (TopicMembersBodyDto, error) {
	if len(input.Members) == 0 {
		return TopicMembersBodyDto{}, &ValidationError{Field: "members", Message: "at least one member is required"}
	}

	members := make([]TopicMemberDto, 0, len(input.Members))

	for index, member := range input.Members {
		if strings.TrimSpace(member.RecipientID) == "" {
			return TopicMembersBodyDto{}, &ValidationError{
				Field:   fmt.Sprintf("members[%d].recipientID", index),
				Message: "must not be empty",
			}
		}

		members = append(members, TopicMemberDto{
			RecipientId: member.RecipientID,
			ExpiresAt:   member.ExpiresAt,
		})
	}

	return TopicMembersBodyDto{
		Members:     members,
		ExpiresAt:   input.ExpiresAt,
		IsTimeBound: input.IsTimeBound,
	}, nil
}

func validateNotify(request NotifyRequest) error {
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return &ValidationError{Field: "idempotencyKey", Message: "must not be empty"}
	}

	if strings.TrimSpace(request.TemplateKey) == "" {
		return &ValidationError{Field: "templateKey", Message: "must not be empty"}
	}

	if request.Data == nil {
		return &ValidationError{Field: "data", Message: "must not be nil"}
	}

	if len(request.Recipients) == 0 && len(request.Topics) == 0 {
		return &ValidationError{Field: "recipients", Message: "recipients or topics are required"}
	}

	for index, recipient := range request.Recipients {
		if strings.TrimSpace(recipient.ID) == "" {
			return &ValidationError{
				Field:   fmt.Sprintf("recipients[%d].id", index),
				Message: "must not be empty",
			}
		}
	}

	return nil
}

func jsonUnmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}
