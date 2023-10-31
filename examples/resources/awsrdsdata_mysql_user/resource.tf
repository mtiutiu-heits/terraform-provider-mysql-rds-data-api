resource "awsrdsdata_mysql_user" "account" {
  user                  = "test"
  host                  = "%"
  password              = "test123456789012" # <- do not store passwords in clear text (use AWS secrets instead)
  database_resource_arn = "arn:aws:rds:us-east-1:777777777777:cluster:test-rds-cluster"
  database_secret_arn   = "arn:aws:secretsmanager:us-east-1:777777777777:secret:test-db-credentials"
}
