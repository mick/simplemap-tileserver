# Both APIs use the same service account (for now)
module "tileserver_service_accounts" {
  source     = "terraform-google-modules/service-accounts/google"
  version    = "~> 3.0"
  project_id = local.project
  names      = ["tileserver"]
  project_roles = [
    "${local.project}=>roles/storage.objectViewer",
    "${local.project}=>roles/logging.logWriter",
    "${local.project}=>roles/monitoring.metricWriter",
  ]
}
