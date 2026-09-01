---
dusk: v1alpha1
namespace: stout
kind: service
name: bookings
title: Bookings
relations:
  - type: themed_for
    to: service:stout/nerdswhofish
attributes:
  environment: undeployed
  language: go
  license: MIT
  repository: NerdsWhoFish/bookings
  runtime: gcp-cloud-run
---

Bookings is a small, themeable scheduling service for Google Calendar. It runs as one request-driven Cloud Run service, stores configuration and booking locks in Firestore, and deliberately has no reminder worker.

The first bundled theme is Nerds Who Fish. Presentation is selected at deployment time from compiled, validated theme packages. Theme code cannot inject arbitrary scripts, HTML, or CSS.

## Data and credentials

Google OAuth client credentials and the session signing key live in Secret Manager. OAuth refresh tokens are encrypted with Cloud KMS before Firestore persistence. The runtime service account receives only Firestore access, KMS encrypt/decrypt access for the token key, Secret Manager access for runtime secrets, and telemetry permissions required by the deployment.

## Operations

The reusable OpenTofu environment module supports a bootstrap deployment with no image or secret payloads. Populate the secret containers, publish an immutable image digest, then apply with that digest to create Cloud Run.

Cloud Run uses request-based billing with zero minimum instances. Availability reads Google free/busy live. Booking claims use five-minute Firestore lock buckets and deterministic Google event IDs so retries cannot double-book a slot.

## Gotchas

- Keep Firestore struct tags explicit. JSON tags do not control Firestore persistence.
- Do not add a worker, scheduler, or queue unless a correctness requirement cannot be met in the request path.
- Do not place OAuth tokens or general OTLP credentials in browser configuration.
- Google Calendar and Firestore are not one transaction. A failed event creation must release the local slot claim.
