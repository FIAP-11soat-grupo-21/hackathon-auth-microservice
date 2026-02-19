output "auth_id" {
  value       = aws_apigatewayv2_authorizer.authorizer.id
  description = "id do authorizer Cognito para a API Gateway"
}