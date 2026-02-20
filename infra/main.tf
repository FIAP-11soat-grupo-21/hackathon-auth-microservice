data "aws_region" "current" {}

module "jwt_function" {
  source = "git::https://github.com/FIAP-11soat-grupo-21/infra-core.git//modules/Lambda?ref=main"

  api_id      = data.terraform_remote_state.api_gateway.outputs.api_id
  lambda_name = "auth-jwt-function"
  handler     = "bootstrap"
  runtime     = "provided.al2023"
  subnet_ids  = data.terraform_remote_state.network_vpc.outputs.private_subnets
  environment = merge(
    var.lambda_environment_variables,
    {
      COGNITO_CLIENT_ID     = data.terraform_remote_state.cognito.outputs.user_pool_client_id
      COGNITO_CLIENT_SECRET = data.terraform_remote_state.cognito.outputs.user_pool_client_secret
      COGNITO_USER_POOL_ID  = data.terraform_remote_state.cognito.outputs.user_pool_id
    }
  )
  vpc_id      = data.terraform_remote_state.network_vpc.outputs.vpc_id
  memory_size = 512
  timeout     = 30

  s3_bucket = var.bucket_name
  s3_key    = "lambda-function.zip"

  role_permissions = {
    cognito = {
      actions = [
        "cognito-idp:AdminInitiateAuth",
        "cognito-idp:AdminUserGlobalSignOut",
        "cognito-idp:ListUsers",
        "cognito-idp:AdminGetUser"
      ]
      resources = ["*"]
    },
    ssm = {
      actions = [
        "ssm:GetParameter",
        "ssm:GetParameters"
      ]
      resources = [
        "*"
      ]
    }
  }
}

resource "aws_apigatewayv2_route" "lambda_route" {
  api_id    = data.terraform_remote_state.api_gateway.outputs.api_id
  route_key = "POST /auth"
  target    = "integrations/${module.jwt_function.lambda_integration_id}"
}

resource "aws_apigatewayv2_authorizer" "authorizer" {
  api_id           = data.terraform_remote_state.api_gateway.outputs.api_id
  name             = "CognitoAuthorizer"
  authorizer_type  = "JWT"
  identity_sources = ["$request.header.Authorization"]
  jwt_configuration {
    audience = [data.terraform_remote_state.cognito.outputs.user_pool_client_id]
    issuer   = "https://cognito-idp.${data.aws_region.current.name}.amazonaws.com/${data.terraform_remote_state.cognito.outputs.user_pool_id}"
  }
}