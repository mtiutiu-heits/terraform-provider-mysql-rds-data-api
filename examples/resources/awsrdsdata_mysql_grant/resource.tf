resource "awsrdsdata_mysql_grant" "permissions" {
  user                  = "test"
  host                  = "%"
  database              = "integration_test"
  privileges            = ["SELECT", "INSERT", "UPDATE"]
  database_resource_arn = "arn:aws:rds:us-east-1:777777777777:cluster:test-rds-cluster"
  database_secret_arn   = "arn:aws:secretsmanager:us-east-1:777777777777:secret:test-db-credentials"
}
