variable "repo" {
  type = object({
    images = list(object({
      name = string
      location = string
    }))
    templates = list(object({
      name = string
      location = string
    }))
  })
}

variable "storage_credentials" {
  type = object({
    host = string
    user = string
    password = string
    zapi_version = string
  })
}

variable "pass_phrase" {
  type = string
}
