provider "awsrdsdata" {
  region = "us-east-1"
}

resource "random_password" "master_password" {
  length  = 16
  special = false
}

resource "aws_secretsmanager_secret" "db_credentials" {
  name = "master_db_credentials"
}

resource "aws_secretsmanager_secret_version" "db_credentials" {
  secret_id = aws_secretsmanager_secret.db_credentials.id
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
}

resource "awsrdsdata_mysql_user" "account" {
  user                  = "test"
  host                  = "%"
  password              = "test123456789012333"
  database_resource_arn = aws_rds_cluster.default.arn
  database_secret_arn   = aws_secretsmanager_secret.db_credentials.arn
}

resource "awsrdsdata_mysql_grant" "permissions" {
  user                  = awsrdsdata_mysql_user.account.user
  host                  = awsrdsdata_mysql_user.account.host
  database              = "test"
  privileges            = ["SELECT", "INSERT", "UPDATE"]
  database_resource_arn = aws_rds_cluster.default
  database_secret_arn   = aws_secretsmanager_secret.db_credentials.arn
}
