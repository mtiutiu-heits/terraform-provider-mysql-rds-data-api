resource "awsrdsdata_mysql_grant" "permissions" {
  user                  = awsrdsdata_mysql_user.account.user
  host                  = awsrdsdata_mysql_user.account.host
  database              = "integration_test"
  privileges            = ["SELECT", "INSERT", "UPDATE"]
  database_resource_arn = aws_rds_cluster.default.arn
  database_secret_arn   = aws_secretsmanager_secret.db_credentials.arn
}
