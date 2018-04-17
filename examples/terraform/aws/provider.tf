# ------------------------------------------------------------------------------
# CONFIGURE OUR AWS CONNECTION
# ------------------------------------------------------------------------------

provider "aws" {
  version = "~> 1.13"

  region = "${var.region}"
}

# ---------------------------------------------------------------------------------------------------------------------
# GET THE LIST OF AVAILABILITY ZONES IN THE CURRENT REGION
# Every AWS accout has slightly different availability zones in each region. For example, one account might have
# us-east-1a, us-east-1b, and us-east-1c, while another will have us-east-1a, us-east-1b, and us-east-1d. This resource
# queries AWS to fetch the list for the current account and region.
# ---------------------------------------------------------------------------------------------------------------------

data "aws_availability_zones" "available" {}

# Use this data source to get the access to the effective Account ID, User ID, and ARN in which Terraform is authorized.
data "aws_caller_identity" "current" {}

# Lookup the AMI based on the owner and name (Ubuntu)
data "aws_ami" "linux_ami" {
  most_recent = true

  filter {
    name   = "owner-id"
    values = ["${var.linux_ami_owner}"]
  }

  filter {
    name   = "name"
    values = ["${var.linux_ami_name}"]
  }
}
