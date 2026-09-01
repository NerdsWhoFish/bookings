provider "google" {
  project = var.project_id
  region  = var.region
}

module "bookings" {
  source = "../../modules/environment"

  project_id        = var.project_id
  region            = var.region
  environment       = "production"
  github_repository = "NerdsWhoFish/bookings"

  image_digest           = var.image_digest
  release_version        = var.release_version
  public_url             = var.public_url
  admin_emails           = var.admin_emails
  google_oauth_client_id = var.google_oauth_client_id
  turnstile_site_key     = var.turnstile_site_key

  external_blocks_enabled = var.external_blocks_enabled

  billing_account_id        = var.billing_account_id
  budget_notification_email = var.budget_notification_email

  faro_url                    = var.faro_url
  otel_exporter_otlp_endpoint = var.otel_exporter_otlp_endpoint
}
