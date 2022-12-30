terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "3.65.0"
    }

    google-beta = {
      source  = "hashicorp/google-beta"
      version = "3.65.0"
    }

  }

  backend "gcs" {
    bucket = "simplemapco-terraform"
    prefix = "terraform/tileserver/state"
  }
}

provider "google" {
  project = local.project
  region  = local.region
  zone    = local.zone
}

provider "google-beta" {
  project = local.project
  region  = local.region
  zone    = local.zone
}
