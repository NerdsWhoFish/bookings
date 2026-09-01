variable "project_id" {
  description = "Existing GCP project ID. Project creation and billing attachment remain outer-root concerns."
  type        = string

  validation {
    condition     = length(trimspace(var.project_id)) > 0
    error_message = "project_id must not be empty."
  }
}

variable "environment" {
  description = "Deployment environment."
  type        = string
  default     = "production"

  validation {
    condition     = contains(["test", "production"], var.environment)
    error_message = "environment must be test or production."
  }
}

variable "region" {
  description = "Cloud Run and Artifact Registry region."
  type        = string
  default     = "us-east1"
}

variable "firestore_location" {
  description = "Immutable Firestore location. Null uses region."
  type        = string
  default     = null
  nullable    = true
}

variable "firestore_database" {
  description = "Firestore database name."
  type        = string
  default     = "(default)"
}

variable "image_digest" {
  description = "Immutable container reference. Null creates bootstrap resources without Cloud Run."
  type        = string
  default     = null
  nullable    = true

  validation {
    condition     = var.image_digest == null || can(regex("@sha256:[0-9a-f]{64}$", var.image_digest))
    error_message = "image_digest must be null or an immutable @sha256 reference."
  }
}

variable "release_version" {
  description = "Human-readable release version exposed to telemetry."
  type        = string
  default     = "dev"
}

variable "github_repository" {
  description = "GitHub owner/name allowed to publish release images."
  type        = string
  default     = "NerdsWhoFish/bookings"

  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", var.github_repository))
    error_message = "github_repository must use owner/name form."
  }
}

variable "public_url" {
  description = "Public HTTPS origin used for OAuth callbacks. Required when image_digest is set."
  type        = string
  default     = null
  nullable    = true

  validation {
    condition     = var.public_url == null || can(regex("^https://[^/]+/?$", var.public_url))
    error_message = "public_url must be null or an HTTPS origin without a path."
  }
}

variable "theme" {
  description = "Compiled theme manifest selected at runtime."
  type        = string
  default     = "nerdswhofish"
}

variable "admin_emails" {
  description = "Google email addresses allowed to bootstrap an administrator session."
  type        = set(string)
  default     = []

  validation {
    condition     = alltrue([for email in var.admin_emails : can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", email))])
    error_message = "admin_emails must contain valid email addresses."
  }
}

variable "google_oauth_client_id" {
  description = "Google OAuth web client ID. The client secret remains in Secret Manager."
  type        = string
  default     = null
  nullable    = true
}

variable "turnstile_site_key" {
  description = "Public Cloudflare Turnstile site key."
  type        = string
  default     = null
  nullable    = true
}

variable "faro_url" {
  description = "Optional public Grafana Faro collector URL."
  type        = string
  default     = null
  nullable    = true
}

variable "faro_app_name" {
  description = "Application name reported by browser telemetry."
  type        = string
  default     = "bookings"
}

variable "otel_exporter_otlp_endpoint" {
  description = "Optional server-side OTLP HTTP endpoint. Headers remain in Secret Manager."
  type        = string
  default     = null
  nullable    = true
}

variable "external_blocks_enabled" {
  description = "Expose the bearer-authenticated API for externally pushed busy blocks."
  type        = bool
  default     = false
}

variable "max_instances" {
  description = "Hard Cloud Run surge and cost cap."
  type        = number
  default     = 3

  validation {
    condition     = var.max_instances >= 1 && var.max_instances <= 20
    error_message = "max_instances must be between 1 and 20."
  }
}

variable "billing_account_id" {
  description = "Optional billing account ID used to create a project-scoped budget."
  type        = string
  default     = null
  nullable    = true
}

variable "monthly_budget_usd" {
  description = "Monthly budget amount when billing_account_id is set."
  type        = number
  default     = 5

  validation {
    condition     = var.monthly_budget_usd >= 1
    error_message = "monthly_budget_usd must be at least 1."
  }
}

variable "budget_notification_email" {
  description = "Optional email notification channel for the budget."
  type        = string
  default     = null
  nullable    = true
}

variable "monitoring_notification_channels" {
  description = "Existing Monitoring channel resource names for service alerts."
  type        = set(string)
  default     = []
}

variable "labels" {
  description = "Additional labels merged onto supported resources."
  type        = map(string)
  default     = {}
}
