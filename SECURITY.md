# Security policy

## Supported versions

Security fixes are made on the current release line. Deployments should use an immutable image digest from the latest stable release.

## Report a vulnerability

Use GitHub's private vulnerability reporting for this repository. Do not open a public issue with exploit details, credentials, tokens, calendar data, or personal information.

Include the affected version, the shortest reproduction you can provide, and the impact you observed. Please remove real OAuth tokens and guest data from screenshots and logs.

## Deployment boundary

Operators own the GCP project, Google OAuth consent configuration, DNS, Turnstile site, Grafana collectors, secret payloads, and public access policy. The OpenTofu module creates the application resources and least-privilege identities but does not create or attach a billing account.
