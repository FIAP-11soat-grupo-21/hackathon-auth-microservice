provider "aws" {
  region = "us-east-2"
}

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.1"
    }
  }

  backend "s3" {
    bucket = "fiap-tc-terraform-846874"
    key    = "tech-challenge-project/auth/terraform.tfstate"
    region = "us-east-2"
  }
}