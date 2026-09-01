locals {
  name_prefix        = "bookings-${var.environment}"
  deploy_service     = var.image_digest != null
  firestore_location = coalesce(var.firestore_location, var.region)
  labels = merge({
    application = "bookings"
    environment = var.environment
    managed_by  = "opentofu"
  }, var.labels)
  required_apis = toset([
    "artifactregistry.googleapis.com",
    "billingbudgets.googleapis.com",
    "cloudkms.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "firestore.googleapis.com",
    "iam.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "serviceusage.googleapis.com",
  ])
  secret_ids = {
    google_oauth_client_secret = "${local.name_prefix}-google-oauth-client-secret"
    session_key                = "${local.name_prefix}-session-key"
    turnstile_secret           = "${local.name_prefix}-turnstile-secret"
    otel_exporter_headers      = "${local.name_prefix}-otel-exporter-headers"
  }
}

check "runtime_configuration" {
  assert {
    condition = !local.deploy_service || (
      var.public_url != null &&
      var.google_oauth_client_id != null &&
      var.turnstile_site_key != null &&
      length(var.admin_emails) > 0
    )
    error_message = "A deployed service requires public_url, google_oauth_client_id, turnstile_site_key, and at least one admin email."
  }
}

resource "google_project_service" "required" {
  for_each = local.required_apis

  project                    = var.project_id
  service                    = each.value
  disable_on_destroy         = false
  disable_dependent_services = false
}

data "google_project" "current" {
  project_id = var.project_id
}

resource "google_artifact_registry_repository" "bookings" {
  project       = var.project_id
  location      = var.region
  repository_id = "bookings"
  description   = "Immutable Bookings application images"
  format        = "DOCKER"
  labels        = local.labels

  depends_on = [google_project_service.required]
}

resource "google_firestore_database" "bookings" {
  project                           = var.project_id
  name                              = var.firestore_database
  location_id                       = local.firestore_location
  type                              = "FIRESTORE_NATIVE"
  concurrency_mode                  = "OPTIMISTIC"
  app_engine_integration_mode       = "DISABLED"
  point_in_time_recovery_enablement = var.environment == "production" ? "POINT_IN_TIME_RECOVERY_ENABLED" : "POINT_IN_TIME_RECOVERY_DISABLED"
  delete_protection_state           = var.environment == "production" ? "DELETE_PROTECTION_ENABLED" : "DELETE_PROTECTION_DISABLED"
  deletion_policy                   = "DELETE"

  depends_on = [google_project_service.required]
}

resource "google_firestore_field" "slot_lock_ttl" {
  project    = var.project_id
  database   = google_firestore_database.bookings.name
  collection = "slot_locks"
  field      = "expires_at"

  ttl_config {}
}

resource "google_kms_key_ring" "bookings" {
  project  = var.project_id
  name     = local.name_prefix
  location = var.region

  depends_on = [google_project_service.required]
}

resource "google_kms_crypto_key" "oauth_tokens" {
  name            = "oauth-tokens"
  key_ring        = google_kms_key_ring.bookings.id
  rotation_period = "7776000s"
}

resource "google_secret_manager_secret" "runtime" {
  for_each = local.secret_ids

  project   = var.project_id
  secret_id = each.value
  labels    = local.labels

  replication {
    auto {}
  }

  depends_on = [google_project_service.required]
}

resource "google_service_account" "runtime" {
  project      = var.project_id
  account_id   = "${local.name_prefix}-runtime"
  display_name = "Bookings ${var.environment} runtime"
}

resource "google_project_iam_member" "runtime_firestore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = google_service_account.runtime.member
}

resource "google_kms_crypto_key_iam_member" "runtime_tokens" {
  crypto_key_id = google_kms_crypto_key.oauth_tokens.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = google_service_account.runtime.member
}

resource "google_secret_manager_secret_iam_member" "runtime" {
  for_each = google_secret_manager_secret.runtime

  project   = var.project_id
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.runtime.member
}

resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = "${local.name_prefix}-github"
  display_name              = "Bookings GitHub releases"

  depends_on = [google_project_service.required]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-oidc"
  display_name                       = "GitHub Actions OIDC"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }
  attribute_condition = "attribute.repository == \"${var.github_repository}\""
}

resource "google_service_account" "releaser" {
  project      = var.project_id
  account_id   = "${local.name_prefix}-releaser"
  display_name = "Bookings ${var.environment} release publisher"
}

resource "google_artifact_registry_repository_iam_member" "releaser" {
  project    = var.project_id
  location   = var.region
  repository = google_artifact_registry_repository.bookings.repository_id
  role       = "roles/artifactregistry.writer"
  member     = google_service_account.releaser.member
}

resource "google_service_account_iam_member" "releaser" {
  service_account_id = google_service_account.releaser.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repository}"
}

resource "google_cloud_run_v2_service" "bookings" {
  count = local.deploy_service ? 1 : 0

  project             = var.project_id
  name                = local.name_prefix
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = var.environment == "production"

  template {
    service_account                  = google_service_account.runtime.email
    timeout                          = "30s"
    max_instance_request_concurrency = 40

    scaling {
      min_instance_count = 0
      max_instance_count = var.max_instances
    }

    containers {
      image = var.image_digest

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle          = true
        startup_cpu_boost = false
      }

      env {
        name  = "BOOKINGS_GCP_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "BOOKINGS_FIRESTORE_DATABASE"
        value = google_firestore_database.bookings.name
      }
      env {
        name  = "BOOKINGS_KMS_KEY_NAME"
        value = google_kms_crypto_key.oauth_tokens.id
      }
      env {
        name  = "BOOKINGS_PUBLIC_URL"
        value = trimsuffix(var.public_url, "/")
      }
      env {
        name  = "BOOKINGS_THEME"
        value = var.theme
      }
      env {
        name  = "BOOKINGS_ADMIN_EMAILS"
        value = join(",", sort(tolist(var.admin_emails)))
      }
      env {
        name  = "BOOKINGS_GOOGLE_CLIENT_ID"
        value = var.google_oauth_client_id
      }
      env {
        name  = "BOOKINGS_TURNSTILE_SITE_KEY"
        value = var.turnstile_site_key
      }
      env {
        name  = "BOOKINGS_FARO_URL"
        value = var.faro_url == null ? "" : var.faro_url
      }
      env {
        name  = "BOOKINGS_FARO_APP_NAME"
        value = var.faro_app_name
      }
      env {
        name  = "OTEL_SERVICE_NAME"
        value = "bookings"
      }
      env {
        name  = "OTEL_SERVICE_VERSION"
        value = var.release_version
      }
      env {
        name  = "OTEL_EXPORTER_OTLP_ENDPOINT"
        value = coalesce(var.otel_exporter_otlp_endpoint, "")
      }

      dynamic "env" {
        for_each = {
          BOOKINGS_GOOGLE_CLIENT_SECRET = "google_oauth_client_secret"
          BOOKINGS_SESSION_KEY          = "session_key"
          BOOKINGS_TURNSTILE_SECRET     = "turnstile_secret"
        }
        content {
          name = env.key
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.runtime[env.value].secret_id
              version = "latest"
            }
          }
        }
      }

      dynamic "env" {
        for_each = var.otel_exporter_otlp_endpoint == null ? [] : [1]
        content {
          name = "OTEL_EXPORTER_OTLP_HEADERS"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.runtime["otel_exporter_headers"].secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }

  depends_on = [
    google_project_service.required,
    google_secret_manager_secret_iam_member.runtime,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  count = local.deploy_service ? 1 : 0

  project  = var.project_id
  location = google_cloud_run_v2_service.bookings[0].location
  name     = google_cloud_run_v2_service.bookings[0].name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_monitoring_notification_channel" "billing_email" {
  count = var.billing_account_id != null && var.budget_notification_email != null ? 1 : 0

  project      = var.project_id
  display_name = "Bookings ${var.environment} billing"
  type         = "email"
  enabled      = true

  labels = {
    email_address = var.budget_notification_email
  }

  depends_on = [google_project_service.required]
}

resource "google_billing_budget" "bookings" {
  count = var.billing_account_id == null ? 0 : 1

  billing_account = var.billing_account_id
  display_name    = "Bookings ${var.environment}"

  budget_filter {
    projects = ["projects/${data.google_project.current.number}"]
  }

  amount {
    specified_amount {
      currency_code = "USD"
      units         = tostring(var.monthly_budget_usd)
    }
  }

  dynamic "threshold_rules" {
    for_each = toset([0.5, 0.8, 1.0])
    content {
      threshold_percent = threshold_rules.value
    }
  }

  all_updates_rule {
    monitoring_notification_channels = concat(tolist(var.monitoring_notification_channels), google_monitoring_notification_channel.billing_email[*].name)
    enable_project_level_recipients  = true
  }

  deletion_policy = var.environment == "production" ? "PREVENT" : "DELETE"

  depends_on = [google_project_service.required]
}

resource "google_monitoring_alert_policy" "server_errors" {
  count = local.deploy_service ? 1 : 0

  project      = var.project_id
  display_name = "Bookings ${var.environment} HTTP 5xx"
  combiner     = "OR"
  enabled      = true

  conditions {
    display_name = "Cloud Run returned 5xx"

    condition_threshold {
      filter          = "resource.type = \"cloud_run_revision\" AND metric.type = \"run.googleapis.com/request_count\" AND resource.labels.service_name = \"${google_cloud_run_v2_service.bookings[0].name}\" AND metric.labels.response_code_class = \"5xx\""
      comparison      = "COMPARISON_GT"
      threshold_value = 0
      duration        = "0s"

      aggregations {
        alignment_period   = "300s"
        per_series_aligner = "ALIGN_RATE"
      }
    }
  }

  notification_channels = tolist(var.monitoring_notification_channels)

  alert_strategy {
    auto_close = "1800s"
  }

  depends_on = [google_project_service.required]
}
