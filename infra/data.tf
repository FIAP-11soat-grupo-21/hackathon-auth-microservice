data "terraform_remote_state" "api_gateway" {
  backend = "s3"
  config = {
    bucket = "fiap-tc-terraform-846874"
    key    = "tech-challenge-project/GatewayAPI/terraform.tfstate"
    region = "us-east-2"
  }
}

data "terraform_remote_state" "network_vpc" {
  backend = "s3"
  config = {
    bucket = "fiap-tc-terraform-846874"
    key    = "tech-challenge-project/Network/VPC/terraform.tfstate"
    region = "us-east-2"
  }
}

data "terraform_remote_state" "cognito" {
  backend = "s3"
  config = {
    bucket = "fiap-tc-terraform-846874"
    key    = "tech-challenge-project/Cognito/terraform.tfstate"
    region = "us-east-2"
  }
}