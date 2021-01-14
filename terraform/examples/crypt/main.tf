provider "flexbot" {
  pass_phrase = var.pass_phrase
}

data "flexbot_crypt" "rancher_token_key" {
  name = "rancher_token_key"
  decrypted = "token-n6kj2:9ts9mcg<..skip..>dwf5dlcr64rqq9lpfgtyws56n6q9wh"
}

data "flexbot_crypt" "aws_access_key_id" {
  name = "aws_access_key_id"
  decrypted = "ADI<...skip...>FV6D"
}

data "flexbot_crypt" "aws_secret_access_key" {
  name = "aws_secret_access_key"
  decrypted = "8W56fiGH0I<...skip...>LKkEfTddbhsbbn8"
}

data "flexbot_crypt" "infoblox_user" {
  name = "infoblox_user"
  decrypted = "admin"
}

data "flexbot_crypt" "infoblox_password" {
  name = "infoblox_password"
  decrypted = "secret"
}

data "flexbot_crypt" "ucsm_user" {
  name = "ucsm_user"
  decrypted = "admin"
}

data "flexbot_crypt" "ucsm_password" {
  name = "ucsm_password"
  decrypted = "secret"
}

data "flexbot_crypt" "cdot_user" {
  name = "cdot_user"
  decrypted = "vsadmin"
}

data "flexbot_crypt" "cdot_password" {
  name = "cdot_password"
  decrypted = "secret"
}
