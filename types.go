package notifhub

import "time"

// NotifyRequest is the ergonomic input for Client.Notify.
//
// It maps to POST /api/v1/notify. Recipients accepts either an ID-only recipient or
// an inline override with email/phone/locale/timezone/metadata.
type NotifyRequest struct {
	IdempotencyKey string
	TemplateKey    string
	Recipients     []Recipient
	Topics         []string
	Channels       []string
	Data           map[string]any
	Priority       string
	TenantID       string
	ScheduledAt    *time.Time
}

// Recipient identifies a notification recipient, optionally with inline overrides.
type Recipient struct {
	ID       string
	Email    *string
	Phone    *string
	Locale   *string
	Timezone *string
	Metadata map[string]any
}

// NotifyResult is the accepted ingest response.
type NotifyResult struct {
	NotificationID string
	Status         string
}

// SubscriberInput is the payload for Client.UpsertSubscriber.
type SubscriberInput struct {
	Email    *string
	Phone    *string
	Locale   *string
	Timezone *string
	Metadata map[string]any
}

// TopicMemberInput describes one topic membership entry.
type TopicMemberInput struct {
	RecipientID string
	ExpiresAt   *time.Time
}

// TopicMembersInput is the payload for replace/add membership calls.
type TopicMembersInput struct {
	Members     []TopicMemberInput
	ExpiresAt   *time.Time
	IsTimeBound *bool
}

// Generated model aliases kept stable for SDK consumers.
type (
	Subscriber    = SubscriberResponseDto
	TopicMember   = TopicMemberViewDto
	TopicMembers  = TopicMembersResponseDto
	TopicMutation = TopicMutationResponseDto
)
