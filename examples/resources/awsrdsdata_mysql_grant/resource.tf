# Provision credentials for the MySQL DB acount used to test the provider
resource "random_password" "test_account_password" {
  length  = 16
  special = false
}

resource "awsrdsdata_mysql_user" "test_account" {
  user                  = "test"
  host                  = "%"
  password              = random_password.test_account_password.result
  database_resource_arn = "<YOUR_MYSQL_RDS_CLUSTER_ARN_HERE>"
  database_secret_arn   = "<YOUR_MYSQL_RDS_CLUSTER_MASTER_CREDENTIALS_AWS_SECRET_ARN_HERE>"
}

resource "awsrdsdata_mysql_grant" "permissions" {
  user                  = awsrdsdata_mysql_user.test_account.user
  host                  = awsrdsdata_mysql_user.test_account.host
  database              = "<YOUR_MYSQL_DATABASE_NAME_HERE>"
  privileges            = ["SELECT", "INSERT", "UPDATE"]
  database_resource_arn = "<YOUR_MYSQL_RDS_CLUSTER_ARN_HERE>"
  database_secret_arn   = "<YOUR_MYSQL_RDS_CLUSTER_MASTER_CREDENTIALS_AWS_SECRET_ARN_HERE>"
}
