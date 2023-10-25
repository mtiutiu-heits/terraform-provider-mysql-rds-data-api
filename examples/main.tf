terraform {
  required_providers {
    awsrdsdata = {
      source = "hashicorp.com/mtiutiu-heits/awsrdsdata"
    }
  }
}

provider "awsrdsdata" {
  # AWS region (optional)
  region = "us-east-1"
}

resource "awsrdsdata_mysql_user" "account" {
  user                  = "test"
  host                  = "%"
  password              = "test123456789012333"
  database_resource_arn = "arn:aws:rds:us-east-1:777777777777:cluster:test-rds-cluster"
  database_secret_arn   = "arn:aws:secretsmanager:us-east-1:777777777777:secret:test-db-credentials"
}

resource "awsrdsdata_mysql_grant" "permissions" {
  user                  = awsrdsdata_mysql_user.account.user
  host                  = awsrdsdata_mysql_user.account.host
  database              = "integration_test"
  privileges            = ["SELECT", "INSERT", "UPDATE"]
  database_resource_arn = awsrdsdata_mysql_user.account.database_resource_arn
  database_secret_arn   = awsrdsdata_mysql_user.account.database_secret_arn
}
