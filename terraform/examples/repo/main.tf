provider "flexbot" {
  pass_phrase = var.pass_phrase
  storage {
    credentials {
      host = var.storage_credentials.host
      user = var.storage_credentials.user
      password = var.storage_credentials.password
      zapi_version = var.storage_credentials.zapi_version
    }
  }
}

# repositories
resource "flexbot_repo" "repo" {
  dynamic "image_repo" {
    for_each = var.repo.images
      content {
        name = image_repo.value.name
        location = image_repo.value.location
      }
  }
  dynamic "template_repo" {
    for_each = var.repo.templates
    content {
      name = template_repo.value.name
      location = template_repo.value.location
    }
  }
}
