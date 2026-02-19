variable "lambda_environment_variables" {
  description = "Variáveis de ambiente da função Lambda"
  type        = map(string)
  default     = {}
}

variable "bucket_name" {
  description = "Nome do bucket S3 para armazenar o código da função Lambda"
  type        = string
  default     = "fiap-tc-terraform-functions-846874"
}