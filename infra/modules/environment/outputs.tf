output "service_url" {
  description = "Cloud Run URI, or null during bootstrap."
  value       = try(google_cloud_run_v2_service.bookings[0].uri, null)
}

output "firestore_database" {
  value = google_firestore_database.bookings.name
}

output "kms_key_name" {
  value = google_kms_crypto_key.oauth_tokens.id
}

output "secret_ids" {
  value = { for key, secret in google_secret_manager_secret.runtime : key => secret.secret_id }
}

output "release_wif_provider" {
  description = "GitHub Actions Workload Identity provider."
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "release_service_account" {
  description = "Service account impersonated by GitHub Actions."
  value       = google_service_account.releaser.email
}

output "release_image" {
  description = "Artifact Registry image name without a tag or digest."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.bookings.repository_id}/bookings"
}

