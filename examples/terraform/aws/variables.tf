###########################################
#
# Infrakit Vars
#
variable "config_root" {
  description = "Root URL of the bootscript"
  default     = "https://raw.githubusercontent.com/chungers/examples/demo/latest/swarm"
}

variable "infrakit" {
  description = "Docker image of Infrakit"
  default     = "infrakit/devbundle:latest"
}

variable "metadata" {
  description = "URL to configure the metadata plugin"
  default     = "https://infrakit.github.io/examples/latest/metadata/aws/export.ikt"
}

variable "cluster_size" {
  description = "Size of the cluster"
  default     = "3"
}

## Manager details
variable "instance_type" {
  description = "The instance type for the instances"
  default     = "t2.medium"
}

# Linux nodes disk
variable "volume_size" {
  description = "The volume size in GB for bootstrap"
  default     = "20"
}

# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# ---------------------------------------------------------------------------------------------------------------------

variable "deployment" {
  description = "The deployment name for this stack"
}

variable "region" {
  description = "Location/Region of resources deployed"
  default     = ""
}

variable "linux_user" {
  description = "The account to use for ssh connections"
  default     = "ubuntu"
}

variable "key_name" {
  description = "The name of the key pair to associate with the instance"
}

#
# VPC settings
#

variable "vpc_id" {
  description = "If set, create sub-nets within a pre-existing VPC instead of creating a new one."
  default     = ""
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC created, or for the Docker EE allocation within an existing VPC. Another 4 bits will be used as the subnet ID, so a /22 is about the maximum possible."
  default     = "172.31.0.0/16"
}

# ---------------------------------------------------------------------------------------------------------------------
# OTHER PARAMETERS
# ---------------------------------------------------------------------------------------------------------------------

# Cloudstor requirement
variable "efs_supported" {
  description = "Set to '1' if the AWS region supports EFS, or 0 if not (see https://aws.amazon.com/about-aws/global-infrastructure/regional-product-services/)."
  default     = "1"
}

# AMIs
variable "linux_ami_owner" {
  description = "The OwnerID of the Linux AMI (from 'aws ec2 describe-images')"
  default     = "099720109477"
}

variable "linux_ami_name" {
  description = "Linux instances will use the newest AMI matching this pattern"
  default     = "ubuntu/images/hvm-ssd/ubuntu-xenial-16.04-amd64-server-20180306"
}
