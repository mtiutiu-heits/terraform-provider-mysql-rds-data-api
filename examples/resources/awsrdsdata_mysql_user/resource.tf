resource "awsrdsdata_mysql_user" "account" {
  user                  = "test"
  host                  = "%"
  password              = aws_secretsmanager_secret.sql_user.arn
  database_resource_arn = aws_rds_cluster.default.arn
  database_secret_arn   = aws_secretsmanager_secret.db_credentials.arn
}
