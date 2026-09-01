# GCP environment module

This module deploys one Bookings environment into an existing GCP project.

It creates:

- Required service APIs
- One Docker Artifact Registry repository
- Firestore Native with optimistic concurrency and slot-lock TTL
- A KMS key for OAuth tokens
- Secret Manager containers for runtime credentials
- A least-privilege Cloud Run runtime service account
- A GitHub OIDC release publisher with Artifact Registry write access
- One public Cloud Run v2 service when `image_digest` is set
- An HTTP 5xx alert
- An optional project-scoped billing budget

`image_digest = null` is the supported bootstrap mode. It intentionally omits Cloud Run so secret versions can be populated without a dependency cycle. The module never accepts secret payloads because OpenTofu state is the wrong place for them.

See the repository [deployment guide](../../../README.md#deploy-with-opentofu) and [basic example](../../examples/basic).
