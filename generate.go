package notifhub

// Regenerates the HTTP client and models from the committed public OpenAPI spec.
//
// The spec itself is produced from the running backend:
//
//	node scripts/export-openapi.mjs
//	cd sdk/go && go generate ./...
//
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 -config oapi-codegen.yaml ../../openapi/notifhub.json
