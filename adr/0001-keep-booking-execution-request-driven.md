# 1. Keep booking execution request driven

Date: 2026-09-01

## Status

Accepted.

## Context and Problem Statement

The service must support multiple Google calendars and meeting types while retaining near-zero idle cost.
There are no reminder, workflow, or polling requirements that justify an always-on worker.

## Considered Options

1. One request-driven Go service with Firestore and direct Google Calendar API calls
2. Deploy an existing general-purpose scheduler
3. Use PostgreSQL with a separate background worker

## Decision Outcome

Use one Go service for the public booking flow, administrator flow, OAuth callbacks, and static frontend. Store configuration, bookings, rate limits, and bounded slot locks in Firestore. Read Google free/busy data live when showing availability and recheck it before claiming local locks and creating the destination event. Do not add scheduled compute until a feature has a correctness requirement for it.

## Consequences

### Good

- Cloud Run can scale to zero without losing required work
- Firestore avoids an always-on database and provides transactional slot claims
- The deployment has one runtime artifact and no queue or scheduler

### Bad

- Google Calendar and Firestore cannot participate in one atomic transaction, so the booking path needs compensation when event creation fails
- The service is Google-specific until another calendar adapter is deliberately added
- Cold starts affect the first request after idle periods

### Rejected because

- Existing schedulers carry reminder workers, broader feature sets, and deployment assumptions that defeat the narrow scale-to-zero goal.
- PostgreSQL and a worker add baseline cost and operational state without serving a current requirement.
