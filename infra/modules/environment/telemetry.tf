variable "otel_exporter_headers_secrets" {
  description = "Optional existing same-project runtime header secret pinned to a numeric version for an atomic endpoint and credential cutover."
  type = map(object({
    secret_id = string
    version   = string
  }))
  default  = {}
  nullable = false

  validation {
    condition = length(var.otel_exporter_headers_secrets) == 0 || (
      toset(keys(var.otel_exporter_headers_secrets)) == toset(["runtime"]) &&
      can(regex("^https://[^/?#]+(?:/[^?#]*)?$", var.otel_exporter_otlp_endpoint)) &&
      alltrue([for ref in values(var.otel_exporter_headers_secrets) :
        can(regex("^[A-Za-z0-9_-]{1,255}$", ref.secret_id)) &&
        can(regex("^[1-9][0-9]*$", ref.version))
      ])
    )
    error_message = "Provide runtime with a same-project secret ID, numeric version and HTTPS endpoint, or leave the map empty."
  }
}

locals {
  otel_header_references = merge({
    runtime = {
      secret_id = google_secret_manager_secret.runtime["otel_exporter_headers"].secret_id
      version   = "latest"
    }
  }, var.otel_exporter_headers_secrets)
}

# Old revisions retain their original secret and access during rollback.
resource "google_secret_manager_secret_iam_member" "external_otel_headers" {
  for_each = var.otel_exporter_headers_secrets

  project   = var.project_id
  secret_id = each.value.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.runtime.member
}
