// Command send demonstrates the NotifHub Go SDK end to end.
//
// Usage:
//
//	NOTIFHUB_API_KEY=... go run ./examples/send
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hamdanidev/notifhub-go"
)

func main() {
	baseURL := valueOrDefault("NOTIFHUB_BASE_URL", "http://localhost:3000")
	apiKey := os.Getenv("NOTIFHUB_API_KEY")

	if apiKey == "" {
		log.Fatal("NOTIFHUB_API_KEY is required")
	}

	client, err := notifhub.New(
		baseURL,
		apiKey,
		notifhub.WithUserAgent("notifhub-go-example/1.0"),
		notifhub.WithRetry(notifhub.DefaultRetryPolicy()),
	)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	subscriberID := "go-sdk-example"
	email := "go-sdk-example@example.com"
	locale := "id"

	subscriber, err := client.UpsertSubscriber(ctx, subscriberID, notifhub.SubscriberInput{
		Email:  &email,
		Locale: &locale,
	})
	if err != nil {
		log.Fatalf("upsert subscriber: %v", err)
	}

	fmt.Printf("subscriber: %s (%s)\n", subscriber.ExternalId, subscriber.Timezone)

	members, err := client.ReplaceTopicMembers(ctx, "go:example", notifhub.TopicMembersInput{
		Members: []notifhub.TopicMemberInput{{RecipientID: subscriberID}},
	})
	if err != nil {
		log.Fatalf("replace topic members: %v", err)
	}

	fmt.Printf("topic %s: %v members\n", members.Topic.Key, members.MemberCount)

	result, err := client.Notify(ctx, notifhub.NotifyRequest{
		IdempotencyKey: fmt.Sprintf("go-example-%d", time.Now().UnixNano()),
		TemplateKey:    "test.notification",
		Recipients:     []notifhub.Recipient{{ID: subscriberID}},
		Channels:       []string{"inapp"},
		Data:           map[string]any{"nama": "Go SDK"},
	})
	if err != nil {
		log.Fatalf("notify: %v", err)
	}

	fmt.Printf("notification %s: %s\n", result.NotificationID, result.Status)
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
