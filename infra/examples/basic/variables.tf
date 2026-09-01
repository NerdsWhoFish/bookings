variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-east1"
}

variable "image_digest" {
  type     = string
  default  = null
  nullable = true
}

variable "release_version" {
  type    = string
  default = "dev"
}

variable "public_url" {
  type     = string
  default  = null
  nullable = true
}

variable "admin_emails" {
  type    = set(string)
  default = []
}

variable "google_oauth_client_id" {
  type     = string
  default  = null
  nullable = true
}

variable "turnstile_site_key" {
  type     = string
  default  = null
  nullable = true
}

variable "external_blocks_enabled" {
  type    = bool
  default = false
}

variable "billing_account_id" {
  type     = string
  default  = null
  nullable = true
}

variable "budget_notification_email" {
  type     = string
  default  = null
  nullable = true
}

variable "faro_url" {
  type     = string
  default  = null
  nullable = true
}

variable "otel_exporter_otlp_endpoint" {
  type     = string
  default  = null
  nullable = true
}
