# Bookings

A small, themeable scheduling service for Google Calendar.

Bookings covers the useful part of Calendly without dragging in reminders, workflows, payments, or a permanently running database. It runs as one Go service on Cloud Run, checks several Google accounts and calendars for conflicts, and scales to zero when nobody is booking anything.

The first bundled theme is Nerds Who Fish. The software is MIT licensed.

## What it does

- Several meeting types with independent lengths, buffers, notice periods, booking windows, schedules, locations, and destination calendars
- Several Google accounts, with a selectable set of busy calendars on each account
- One-time links for inviting someone else to connect their Google account without giving them administrator access
- Per-meeting attendee selection from connected Google accounts
- Separate private "Busy" invites for work addresses that must never appear on the guest event
- An authenticated push API for busy times from calendars the service cannot read
- Live Google free/busy checks before showing a time and again before booking it
- Transactional five-minute Firestore lock buckets to stop two guests claiming the same time
- Deterministic Google event IDs, Google Meet creation, and immediate guest invitations
- Guest cancellation without an account
- Google OAuth administrator access with an explicit email allowlist
- Cloudflare Turnstile on the public booking form
- OpenTelemetry logs and traces on the server, plus optional Grafana Faro browser telemetry
- A compiled theme contract with no arbitrary CSS, HTML, or script injection
- No reminders

## Shape of the system

The browser, administrator API, OAuth callbacks, and public API ship in one container. Cloud Run uses request-based billing with zero minimum instances. Firestore stores configuration, encrypted OAuth tokens, bookings, and short-lived slot locks. Cloud KMS encrypts every Google token before it reaches Firestore.

Google Calendar and Firestore cannot share a transaction. The booking path handles that gap deliberately:

1. Read free/busy data from every selected calendar.
2. Generate the available slots for the selected meeting type.
3. Recheck the chosen slot.
4. Claim every five-minute bucket covering the meeting and its buffers in one Firestore transaction.
5. Create the Google event with an ID derived from the booking ID.
6. Confirm the local booking. If event creation fails, release the slot claim.

The deterministic event ID makes a retry safe when Google accepts a request but the network drops the response.

The full tradeoffs are in [ADR 0001](adr/0001-keep-booking-execution-request-driven.md), [ADR 0002](adr/0002-compile-themes-as-validated-packages.md), [ADR 0003](adr/0003-use-fragment-carried-invitations-for-delegated-calendar-conn.md), and [ADR 0004](adr/0004-keep-private-work-calendar-blocks-separate-from-guest-events.md).

## Run it locally

Development mode uses an in-memory store, a mock calendar, and three meeting types at 20, 45, and 75 minutes.

```console
npm --prefix web install
npm --prefix web run build
BOOKINGS_DEV_MODE=true go run ./cmd/bookings
```

Open `http://localhost:8080`. The administrator page is at `/admin` and skips OAuth only in development mode.

Run the checks with:

```console
make test
make lint
docker build -t bookings:local .
```

## Deploy with OpenTofu

The module expects an existing GCP project with billing attached. It owns the application resources inside that project.

Pin the module to a release tag:

```hcl
module "bookings" {
  source = "git::https://github.com/NerdsWhoFish/bookings.git//infra/modules/environment?ref=v1.0.0"

  project_id  = "my-bookings-project"
  region      = "us-east1"
  environment = "production"

  public_url             = "https://book.example.com"
  admin_emails           = ["owner@example.com"]
  google_oauth_client_id = "000000000000-example.apps.googleusercontent.com"
  turnstile_site_key     = "0x4AAAA..."

  billing_account_id       = "000000-000000-000000"
  budget_notification_email = "owner@example.com"

  faro_url                    = "https://faro-collector.example.com/collect"
  otel_exporter_otlp_endpoint = "https://otlp-gateway.example.com/otlp"
  external_blocks_enabled     = true
}
```

A complete local-source example is in [infra/examples/basic](infra/examples/basic).

### Bootstrap in two applies

Cloud Run checks Secret Manager access and secret versions while it creates a revision. A first deployment cannot create an empty secret and consume its `latest` version in the same apply.

Start with `image_digest = null`. The first apply creates APIs, Artifact Registry, Firestore, KMS, secret containers, IAM, and the GitHub release identity, but no Cloud Run service.

```console
tofu apply
tofu output -json bookings
```

Add these secret versions outside OpenTofu so their payloads never enter state:

- `google_oauth_client_secret`: Google OAuth web client secret
- `session_key`: at least 32 random bytes
- `turnstile_secret`: Cloudflare Turnstile secret key
- `otel_exporter_headers`: OTLP authentication headers, only when an OTLP endpoint is configured
- `external_block_api_token`: at least 32 random characters, only when `external_blocks_enabled` is true

The module returns their exact Secret Manager IDs in `secret_ids`.

Configure these GitHub repository variables from the module outputs:

| GitHub variable | Module output |
| --- | --- |
| `BOOKINGS_WIF_PROVIDER` | `release_wif_provider` |
| `BOOKINGS_RELEASER_SA` | `release_service_account` |
| `BOOKINGS_AR_IMAGE` | `release_image` |
| `BOOKINGS_AR_REGISTRY` | `<region>-docker.pkg.dev` |

The release workflow uses GitHub OIDC and [Quill](https://github.com/TheOutdoorProgrammer/quill) to publish signed multi-platform images. It stores no service-account key.

After publishing, set `image_digest` to the full immutable reference returned by the release, such as `us-east1-docker.pkg.dev/project/bookings/bookings@sha256:...`, then apply again. Cloud Run is created with request-based billing and zero minimum instances.

### Google OAuth setup

Create a Google OAuth web client and add this exact authorized redirect URI:

```text
https://book.example.com/api/admin/google/callback
```

The app requests email identity, calendar free/busy, event, and calendar-list scopes. A Google Workspace app restricted to one organization can usually remain internal. A public external app may need Google OAuth verification before arbitrary Google accounts can connect. That is provider policy, not something self-hosting avoids.

The first allowed administrator signs in at `/admin`, connects a Google account, selects the calendars that count as busy, and assigns a destination account and calendar to each meeting type.

To add someone else's calendar, enter their Google account email under Connection links and send them the generated link. The link works once and expires after seven days. After they consent through Google, their account appears under Busy calendars and can be selected as an attendee on individual meeting types. Connecting through an invitation does not create an administrator session.

## Meeting types

Each meeting type owns:

- Slug, name, and description
- Duration and slot interval
- Buffer before and after
- Minimum notice and booking-window limits
- IANA time zone and weekly availability
- Location, including Google Meet
- Destination Google account and calendar
- Connected accounts to invite automatically
- Private blocker addresses that each receive a separate sanitized event
- Active, hidden, and deleted states

Hidden meeting types stay off the main booking page but remain available at `/meet/<slug>`. Deleting a meeting type removes it from the administrator and public pages. A storage tombstone keeps existing bookings cancellable without exposing the deleted type.

Private blocker addresses are for calendars that can receive an invitation but must stay separate from the guest. Each address receives its own event named `Busy`. That event has no guest identity, notes, meeting link, or other attendees. If any required blocker event fails, the booking is rolled back and the guest event is canceled.

## External busy blocks

Enable `external_blocks_enabled`, populate the `external_block_api_token` secret, and let a trusted bridge push intervals that should be unavailable. The API stores no event title or attendee data.

Create or update a block with a stable opaque ID:

```console
curl --fail-with-body \
  -X PUT "https://book.example.com/api/external/blocks/work:event-123" \
  -H "Authorization: Bearer $BOOKINGS_EXTERNAL_BLOCK_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"start":"2026-09-02T13:00:00Z","end":"2026-09-02T14:00:00Z"}'
```

Delete it when the source event disappears:

```console
curl --fail-with-body \
  -X DELETE "https://book.example.com/api/external/blocks/work:event-123" \
  -H "Authorization: Bearer $BOOKINGS_EXTERNAL_BLOCK_TOKEN"
```

Both operations are idempotent. IDs may contain letters, numbers, `.`, `_`, `:`, and `-`, up to 128 characters. Use a value derived from the source event ID, not its title, organizer, or attendee addresses. Intervals use RFC 3339 timestamps and half-open `[start, end)` semantics.

Availability is computed in the meeting type's time zone and returned to the guest as absolute timestamps. The browser displays those timestamps in that same zone so daylight-saving transitions stay on the server's time-zone rules.

## Themes

Themes are TypeScript manifests in [web/src/themes.ts](web/src/themes.ts). A theme can set semantic colors, typography-facing shape tokens, and approved copy. It cannot inject raw HTML, JavaScript, or arbitrary CSS.

Add a manifest to the registry, give it a stable ID, and deploy with:

```hcl
theme = "your-theme-id"
```

The shared components, accessibility behavior, booking logic, and security boundaries remain unchanged.

## Cost

At light use, Cloud Run should sit at zero compute cost while idle. Firestore's free quota usually covers a small scheduler. Secret Manager, KMS, Artifact Registry storage, network egress, Firestore point-in-time recovery, and backups can still produce small charges. “Near zero” is honest; “free forever” would be bullshit.

The module defaults to a $5 monthly budget when a billing account ID is provided and caps Cloud Run at three instances. Budgets alert after spend has happened, so they are not a hard billing limit.

## Security notes

- OAuth refresh tokens are encrypted with Cloud KMS and bound to their connection ID as authenticated data.
- Runtime secrets are mounted from Secret Manager and never sent to the browser.
- Browser telemetry receives only the public Faro collector URL and app name. General OTLP credentials stay server-side.
- Administrator mutations require a signed, HTTP-only, same-site cookie and a same-origin request.
- Booking cancellation tokens are random, stored only as SHA-256 hashes, and never logged.
- Calendar connection links are email-bound, expire after seven days, and work once. Only a SHA-256 hash of the secret is stored. The secret stays in the URL fragment so it does not reach Cloud Run access logs or referrer headers.
- External blocks require a separate bearer token from Secret Manager. Their IDs and payloads should contain no calendar content beyond start and end timestamps.
- Turnstile is mandatory outside development mode.
- The content security policy allows only the application, Turnstile, and configured Grafana collector families.

See [SECURITY.md](SECURITY.md) for reporting.

## License

The code is available under the [MIT License](LICENSE). Nerds Who Fish names and marks are covered separately in [TRADEMARKS.md](TRADEMARKS.md).
