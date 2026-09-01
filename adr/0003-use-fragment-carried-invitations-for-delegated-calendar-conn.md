# 3. Use fragment-carried invitations for delegated calendar connections

Date: 2026-09-01

## Status

Accepted.

## Context and Problem Statement

Administrators need to invite another person to connect a Google Calendar without sharing an administrator session.
Invitation credentials must not leak through browser history synchronization, reverse-proxy request targets, Cloud Run access logs, referrer headers, or Firestore plaintext.
The Google OAuth callback still needs durable server-side context after the recipient leaves for Google consent.

## Considered Options

1. Use one-time email-bound invitation tokens in URL fragments and exchange them for a signed short-lived OAuth bridge cookie
2. Put one-time invitation tokens in URL query parameters
3. Require an administrator to start every delegated OAuth connection

## Decision Outcome

Chosen: **option 1**.

Create a random invitation token, persist only its SHA-256 hash, bind it to the intended email address, and give the administrator a link whose credential is stored in the URL fragment. The connect page submits the token in a same-origin POST. After validation, the server issues a signed, HttpOnly, short-lived bridge cookie containing the invitation ID, then starts the existing Google OAuth flow. The callback verifies that the Google account email matches the invitation, consumes the invitation transactionally, stores the encrypted OAuth token, and redirects without issuing an administrator session.

## Consequences

### Good

- Cloud Run, reverse proxies, and referrer headers never receive the invitation credential in a request URL
- A leaked Firestore invitation document does not reveal a usable token
- Invited people can connect their own account without receiving administrator privileges
- Expiration, revocation, email binding, and one-time consumption limit credential misuse

### Bad

- The connect page requires JavaScript to move the fragment credential into a POST body
- The OAuth flow gains another signed cookie and state transition that must be tested
- A recipient must open the original link again if the bridge cookie expires before consent completes

### Rejected because

- Query parameters are simpler, but they expose the credential to request logs, browser history, and more observability surfaces.
- Administrator-initiated OAuth does not solve the remote invitation requirement and encourages credential or session sharing.
