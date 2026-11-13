# GCP environment module

This module deploys one Bookings environment into an existing GCP project.

It creates:

- Required service APIs
- One Docker Artifact Registry repository
- Firestore Native with optimistic concurrency and slot-lock TTL
- A KMS key for OAuth tokens
- Secret Manager containers for runtime credentials
- An optional bearer-authenticated API for externally pushed busy blocks
- A least-privilege Cloud Run runtime service account
- A GitHub OIDC release publisher with Artifact Registry write access
- One public Cloud Run v2 service when `image_digest` is set
- An HTTP 5xx alert
- An optional project-scoped billing budget

`image_digest = null` is the supported bootstrap mode. It intentionally omits Cloud Run so secret versions can be populated without a dependency cycle. The module never accepts secret payloads because OpenTofu state is the wrong place for them.

See the repository [deployment guide](../../../README.md#deploy-with-opentofu) and [basic example](../../examples/basic).

## Changing OTLP destinations

For an atomic endpoint and credential cutover, create and populate a separate same-project Secret Manager container first. Set `otel_exporter_headers_secrets = { runtime = { secret_id = "...", version = "1" } }` with the actual numeric version together with `otel_exporter_otlp_endpoint`. No payload belongs in configuration or state.

The default remains the original managed container at `latest`. External references add a container-scoped runtime accessor grant and preserve existing containers and IAM for rollback. Do not overwrite the old latest credential while old Cloud Run revisions still refer to it.

The application retains service name `bookings`, its current logs and traces, and existing privacy controls. This does not add an application metrics provider or change browser RUM. Verify a new revision and correlated telemetry before removing any old credential or monitoring resources.
