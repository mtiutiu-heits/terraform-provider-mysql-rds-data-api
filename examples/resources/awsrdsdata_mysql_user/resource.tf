# Provision credentials for the MySQL DB acount used to test the provider
resource "random_password" "test_account_password" {
  length  = 16
  special = false
}

# Also, store sensitive data in a dedicated AWS secret
resource "aws_secretsmanager_secret" "test_account_db_credentials" {
  name = "test_account_db_credentials"
}

resource "aws_secretsmanager_secret_version" "test_account_db_credentials" {
  secret_id = aws_secretsmanager_secret.test_account_db_credentials.id
  secret_string = jsonencode(
    {
      username = awsrdsdata_mysql_user.test_account.user
      password = awsrdsdata_mysql_user.test_account.password
    }
  )
}

resource "awsrdsdata_mysql_user" "test_account" {
  user                  = "test"
  host                  = "%"
  password              = random_password.test_account_password.result
  database_resource_arn = "<YOUR_MYSQL_RDS_CLUSTER_ARN_HERE>"
  database_secret_arn   = "<YOUR_MYSQL_RDS_CLUSTER_MASTER_CREDENTIALS_AWS_SECRET_ARN_HERE>"
}
