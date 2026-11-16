# NotifHub Go SDK

Official Go client for the NotifHub consumer API. The HTTP client and models are generated
from the committed public OpenAPI spec (`openapi/notifhub.json`); the wrapper adds an ergonomic
surface, typed errors, and optional retries.

Supported endpoints:

- `POST /api/v1/notify`
- `PUT` / `GET /api/v1/subscribers/{externalId}`
- `PUT` / `POST` / `GET /api/v1/topics/{key}/members`

Admin, webhook, and in-app endpoints are intentionally excluded.

## Install

The module is published from the `github.com/hamdanidev/notifhub-go` repository (a release mirror
of this directory). Once a release tag exists:

```bash
go get github.com/hamdanidev/notifhub-go
```

Until then, consume the SDK from a checkout of the main repository with a `replace` directive:

```go
// go.mod of the consuming project
require github.com/hamdanidev/notifhub-go v0.0.0

replace github.com/hamdanidev/notifhub-go => ../NotifHub/sdk/go
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hamdanidev/notifhub-go"
)

func main() {
	client, err := notifhub.New(
		"http://localhost:3000",
		os.Getenv("NOTIFHUB_API_KEY"),
		notifhub.WithRetry(notifhub.DefaultRetryPolicy()),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	email := "user@example.com"
	if _, err := client.UpsertSubscriber(ctx, "user-1", notifhub.SubscriberInput{
		Email:  &email,
		Locale: ptr("id"),
	}); err != nil {
		log.Fatal(err)
	}

	result, err := client.Notify(ctx, notifhub.NotifyRequest{
		IdempotencyKey: "order-123-confirmed",
		TemplateKey:    "order.confirmed",
		Recipients:     []notifhub.Recipient{{ID: "user-1"}},
		Channels:       []string{"inapp", "email"},
		Data:           map[string]any{"order_id": "123"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.NotificationID, result.Status)

	members, err := client.ReplaceTopicMembers(ctx, "shift:gudang", notifhub.TopicMembersInput{
		Members: []notifhub.TopicMemberInput{{RecipientID: "user-1"}},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(members.MemberCount)
}

func ptr[T any](value T) *T { return &value }
```

## Errors

```go
var apiErr *notifhub.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.StatusCode, apiErr.Message)
}

if notifhub.IsUnauthorized(err) {
	// missing/invalid X-API-Key
}
```

Helpers: `IsNotFound`, `IsUnauthorized`, `IsRateLimited`, `IsValidation`.

## Retries

Retries are disabled by default. Enable them with `WithRetry`:

```go
client, err := notifhub.New(baseURL, apiKey, notifhub.WithRetry(notifhub.RetryPolicy{
	MaxRetries: 3,
	BaseDelay:  250 * time.Millisecond,
	MaxDelay:   2 * time.Second,
}))
```

Only `429` and `5xx` responses are retried. `Notify` is retried **only** when
`IdempotencyKey` is set; other calls (`GET`, `PUT`, topic membership) are retried because they
are idempotent.

## Options

- `WithHTTPDoer(*http.Client)` — custom client/transport (default timeout 30s).
- `WithUserAgent("order-service/1.2")`.
- `WithRetry(policy)`.

## Regenerating the client

```bash
# from the repository root, with the backend running
node scripts/export-openapi.mjs
cd sdk/go && go generate ./...
```

`scripts/export-openapi.mjs` filters `/api/docs-json` down to the public paths and their
transitive schemas. `generate.go` pins `oapi-codegen` (`v2.4.1`, compatible with Go 1.23).
Do not edit `client.gen.go` by hand.

## Notes

- `Notify` sends recipients as either `"user-id"` strings or inline override objects
  (`{ "id": ..., "email": ... }`). This avoids the `[][]interface{}` shape that OpenAPI
  generators produce for the string-or-object union.
- Request models exposed by the wrapper (`NotifyRequest`, `Recipient`, `SubscriberInput`,
  `TopicMembersInput`) are thin ergonomic types; response models are generated aliases
  (`Subscriber`, `TopicMembers`, `TopicMutation`).
