variable "services_tag" {
  type    = string
  default = "latest"
}

locals {
  region                = "us-central1"
  zone                  = "us-central1-c"
  min_scale             = "0"
  max_scale             = "2"
  limits_memory         = "1024Mi"
  limits_cpu            = "1000m"
  container_concurrency = 100

  # The main things to config if youre using this terraform to deploy your own on GCP.
  mbtiles_path      = "gs://simplemapco-assets/tilesets/"
  tileserver_domain = "demotiles.simplemap.co"
  project           = "simplemapco"
}
