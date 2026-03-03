variable "lambda_environment_variables" {
  description = "Variáveis de ambiente da função Lambda"
  type        = map(string)
  default     = {}
}

variable "function_name" {
    description = "Nome da função Lambda"
    type        = string
    default     = "my-lambda-function"
}