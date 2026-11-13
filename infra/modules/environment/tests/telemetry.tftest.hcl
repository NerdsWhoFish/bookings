mock_provider "google" {
  mock_resource "google_service_account" {
    defaults = {
      email  = "runtime@bookings-test.iam.gserviceaccount.com"
      member = "serviceAccount:runtime@bookings-test.iam.gserviceaccount.com"
      name   = "projects/bookings-test/serviceAccounts/runtime@bookings-test.iam.gserviceaccount.com"
    }
  }
}

variables {
  project_id                  = "bookings-test"
  environment                 = "test"
  image_digest                = "us-east1-docker.pkg.dev/bookings-test/bookings/bookings@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  public_url                  = "https://book.example.com"
  google_oauth_client_id      = "test.apps.googleusercontent.com"
  turnstile_site_key          = "test-site-key"
  admin_emails                = ["owner@example.com"]
  otel_exporter_otlp_endpoint = "https://otel.example.com"
}

run "legacy_reference_is_unchanged" {
  command = plan
  assert {
    condition = alltrue([
      for env in google_cloud_run_v2_service.bookings[0].template[0].containers[0].env :
      (
        env.value_source[0].secret_key_ref[0].secret == google_secret_manager_secret.runtime["otel_exporter_headers"].secret_id &&
        env.value_source[0].secret_key_ref[0].version == "latest"
      ) if env.name == "OTEL_EXPORTER_OTLP_HEADERS"
    ]) && length(google_secret_manager_secret_iam_member.external_otel_headers) == 0
    error_message = "Existing callers must preserve the legacy header reference and IAM."
  }
}

run "external_reference_is_pinned_and_scoped" {
  command = plan
  variables {
    otel_exporter_headers_secrets = {
      runtime = { secret_id = "local-bookings", version = "4" }
    }
  }
  assert {
    condition = alltrue([
      for env in google_cloud_run_v2_service.bookings[0].template[0].containers[0].env :
      (
        env.value_source[0].secret_key_ref[0].secret == "local-bookings" &&
        env.value_source[0].secret_key_ref[0].version == "4"
      ) if env.name == "OTEL_EXPORTER_OTLP_HEADERS"
    ])
    error_message = "The deployed revision must pin its external credential."
  }
  assert {
    condition = (
      google_secret_manager_secret_iam_member.external_otel_headers["runtime"].secret_id == "local-bookings" &&
      google_secret_manager_secret_iam_member.external_otel_headers["runtime"].member == google_service_account.runtime.member &&
      contains(keys(google_secret_manager_secret.runtime), "otel_exporter_headers") &&
      contains(keys(google_secret_manager_secret_iam_member.runtime), "otel_exporter_headers")
    )
    error_message = "External access must be container-scoped and retain the old credential for rollback."
  }
}

run "reject_cross_project_reference" {
  command = plan
  variables {
    otel_exporter_headers_secrets = {
      runtime = { secret_id = "projects/other/secrets/headers", version = "4" }
    }
  }
  expect_failures = [var.otel_exporter_headers_secrets]
}

run "reject_mutable_version" {
  command = plan
  variables {
    otel_exporter_headers_secrets = {
      runtime = { secret_id = "local-bookings", version = "latest" }
    }
  }
  expect_failures = [var.otel_exporter_headers_secrets]
}

run "reject_plaintext_credential_transport" {
  command = plan
  variables {
    otel_exporter_otlp_endpoint = "http://otel.example.com"
    otel_exporter_headers_secrets = {
      runtime = { secret_id = "local-bookings", version = "4" }
    }
  }
  expect_failures = [var.otel_exporter_headers_secrets]
}
