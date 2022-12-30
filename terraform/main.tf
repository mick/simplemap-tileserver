resource "google_artifact_registry_repository" "tileserver_artifact_registry_repository" {
  provider      = google-beta
  project       = local.project
  location      = "us-central1"
  repository_id = "tileserver"
  description   = "Tileserver"
  format        = "DOCKER"
}

resource "google_cloud_run_service" "tileserver" {
  name     = "remote-tileserver"
  location = local.region

  template {
    spec {
      service_account_name = module.tileserver_service_accounts.email

      container_concurrency = local.container_concurrency
      containers {

        image = "us-central1-docker.pkg.dev/${local.project}/${google_artifact_registry_repository.tileserver_artifact_registry_repository.repository_id}/tileserver:${var.services_tag}"

        resources {
          limits = {
            memory : local.limits_memory
            cpu : local.limits_cpu
          }
        }

        env {
          name  = "VERSION"
          value = var.services_tag
        }

        env {
          name  = "TILESERVER_URL"
          value = "https://${local.tileserver_domain}"
        }

        env {
          name  = "MBTILES_PATH"
          value = local.mbtiles_path
        }
      }
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale" = local.min_scale
        "autoscaling.knative.dev/maxScale" = local.max_scale
        "run.googleapis.com/client-name"   = "terraform"
      }
    }
  }

  metadata {
    annotations = {
      "run.googleapis.com/ingress" = "all"
    }
  }

  autogenerate_revision_name = true
}

data "google_iam_policy" "noauth" {
  binding {
    role = "roles/run.invoker"
    members = [
      "allUsers",
    ]
  }
}

resource "google_cloud_run_service_iam_policy" "tileserver" {
  location = google_cloud_run_service.tileserver.location
  project  = google_cloud_run_service.tileserver.project
  service  = google_cloud_run_service.tileserver.name

  policy_data = data.google_iam_policy.noauth.policy_data
}


resource "google_cloud_run_domain_mapping" "tileserver_custom_domain" {
  location = "us-central1"
  name     = local.tileserver_domain

  metadata {
    namespace = local.project
  }

  spec {
    route_name = google_cloud_run_service.tileserver.name
  }
}
