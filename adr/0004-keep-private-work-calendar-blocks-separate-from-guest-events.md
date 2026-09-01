# 4. Keep private work-calendar blocks separate from guest events

Date: 2026-09-01

## Status

Accepted.

## Context and Problem Statement

Some work calendars cannot grant this service read access.
A booked meeting still needs to block that work calendar without exposing the work address, guest identity, notes, or conferencing details to either side.
A local authenticated bridge can observe the work calendar and publish busy intervals, but Cloud Run cannot safely pull from a private local endpoint.

## Considered Options

1. Create sanitized shadow events and accept idempotent busy blocks through an authenticated push API
2. Add the work address to the guest-facing event and pull availability from the local bridge
3. Require direct calendar OAuth or a public calendar feed

## Decision Outcome

Chosen: **option 1**.

Allow each meeting type to declare blocker email addresses. After the guest event is created, create one separate event per blocker address with a generic Busy summary, no guest data, and no other attendee. Treat every configured shadow event as required and compensate the booking if one fails.

Expose bearer-authenticated PUT and DELETE endpoints for external busy blocks. The local bridge chooses stable block IDs and pushes start and end timestamps. Store those intervals in Firestore and merge overlapping future blocks into availability reads. Keep the integration request-driven, with no poller, queue, or public callback into the local network.

## Consequences

### Good

- Guest-facing events never reveal private work addresses
- Shadow events never contain guest identity, notes, meeting links, or other blocker addresses
- The service can honor calendars it cannot authenticate to or read
- Stable block IDs make bridge retries safe without running background compute

### Bad

- Each booking has more external side effects and needs compensation when a shadow event fails
- Email-based shadow blocking depends on the receiving calendar accepting or automatically adding invitations
- The local bridge must keep its pushed blocks current and protect a dedicated API token
- A missed delete can leave a future interval blocked until the interval ends or the bridge repairs it

### Rejected because

- Putting the work address on the guest event leaks attendee information in both directions, while pulling from a local endpoint requires inbound reachability and couples availability latency to a private service.
- Direct OAuth is unavailable for the stated work calendar, and a public feed exposes more calendar data than the scheduler needs.
