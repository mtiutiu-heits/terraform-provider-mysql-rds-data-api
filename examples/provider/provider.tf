provider "awsrdsdata" {
  region = "us-east-1" # optional
}

# Provision credentials for the master DB acount
resource "random_password" "master_password" {
  length  = 16
  special = false
}

# Also, store sensitive data in a dedicated AWS secret
resource "aws_secretsmanager_secret" "master_db_credentials" {
  name = "master_db_credentials"
}

resource "aws_secretsmanager_secret_version" "master_db_credentials" {
  secret_id = aws_secretsmanager_secret.master_db_credentials.id
  secret_string = jsonencode(
    {
      username = aws_rds_cluster.default.master_username
      password = aws_rds_cluster.default.master_password
      host     = aws_rds_cluster.aurora_cluster.endpoint
      port     = aws_rds_cluster.aurora_cluster.port
    }
  )
}

resource "aws_rds_cluster" "default" {
  cluster_identifier      = "aurora-cluster-demo"
  engine                  = "aurora-mysql"
  engine_version          = "5.7.mysql_aurora.2.03.2"
  availability_zones      = ["us-west-2a", "us-west-2b", "us-west-2c"]
  database_name           = "test"
  master_username         = "master"
  master_password         = random_password.master_password.result
  backup_retention_period = 5
  preferred_backup_window = "07:00-09:00"
  enable_http_endpoint    = true # <- this is very important
}

# Provision credentials for the MySQL DB acount used to test the provider
resource "random_password" "test_account_password" {
  length  = 16
  special = false
}

# Also, store sensitive data in a dedicated AWS secret
resource "aws_secretsmanager_secret" "test_account_db_credentials" {
  name = "master_db_credentials"
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
  database_resource_arn = aws_rds_cluster.default.arn
  database_secret_arn   = aws_secretsmanager_secret.master_db_credentials.arn
}

resource "awsrdsdata_mysql_grant" "permissions" {
  user                  = awsrdsdata_mysql_user.test_account.user
  host                  = awsrdsdata_mysql_user.test_account.host
  database              = "test" # <- the database must exist beforehand
  privileges            = ["SELECT", "INSERT", "UPDATE"]
  database_resource_arn = aws_rds_cluster.default.arn
  database_secret_arn   = aws_secretsmanager_secret.master_db_credentials.arn
}
