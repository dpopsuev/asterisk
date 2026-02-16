// Package rp provides a scope-based client for the Report Portal 5.11 API.
//
// Usage:
//
//	client, err := rp.New(baseURL, token, rp.WithTimeout(30*time.Second))
//	launch, err := client.Project("ecosystem-qe").Launches().Get(ctx, 33195)
//	items, err := client.Project("ecosystem-qe").Items().List(ctx, rp.WithLaunchID(33195), rp.WithStatus("FAILED"))
//	env, err := client.Project("ecosystem-qe").FetchEnvelope(ctx, 33195)
//
// Patterns adapted from report-portal-cli (read-only reference, no module dependency).
// See .cursor/contracts/rp-adapter-v2.md for the design rationale.
package rp
