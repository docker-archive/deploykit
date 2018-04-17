# Logic to determine if we need to create the VPC
locals {
  create_vpc = "${length(var.vpc_id) == 0 ? 1 : 0}"
  vpc_id     = "${local.create_vpc ? join("", aws_vpc.private.*.id) : var.vpc_id}"
}

# Create the stack VPC
resource "aws_vpc" "private" {
  count                = "${local.create_vpc}"
  cidr_block           = "${var.vpc_cidr}"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${format("%s-vpc", "${var.deployment}")}"
  }
}

data "aws_vpc" "selected" {
  id = "${local.vpc_id}"
}

# Create the associated subnet - Need to loop based on Count of AZ
# CIDR block mapping from vars
resource "aws_subnet" "pubsubnet" {
  vpc_id = "${local.vpc_id}"
  count  = "${length("${data.aws_availability_zones.available.names}")}"

  cidr_block              = "${cidrsubnet("${data.aws_vpc.selected.cidr_block}", 4, count.index)}"
  map_public_ip_on_launch = true
  availability_zone       = "${element(data.aws_availability_zones.available.names, count.index)}"

  tags = {
    Name = "${format("%s-Subnet-%d", "${var.deployment}", count.index + 1)}"
  }
}

locals {
  bootstrap_subnet_cidr = "${element("${aws_subnet.pubsubnet.*.cidr_block}", 0)}"
  bootstrap_subnet_id   = "${element(aws_subnet.pubsubnet.*.id, 0)}"
  bootstrap_ip          = "${cidrhost(local.bootstrap_subnet_cidr, 101)}"
}

## Public route table association
resource "aws_route_table_association" "public" {
  count          = "${length("${data.aws_availability_zones.available.names}") * local.create_vpc}"
  subnet_id      = "${element(aws_subnet.pubsubnet.*.id, count.index)}"
  route_table_id = "${aws_route_table.public_igw.id}"
}

resource "aws_internet_gateway" "igw" {
  vpc_id = "${local.vpc_id}"
  count  = "${local.create_vpc}"

  tags = {
    Name = "${format("%s-igw", "${var.deployment}")}"
  }
}

resource "aws_route_table" "public_igw" {
  vpc_id = "${local.vpc_id}"
  count  = "${local.create_vpc}"

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = "${aws_internet_gateway.igw.id}"
  }

  tags {
    Name = "${format("%s-rt", "${var.deployment}")}"
  }
}

resource "aws_route" "internet_access" {
  count                  = "${local.create_vpc}"
  route_table_id         = "${aws_route_table.public_igw.id}"
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = "${aws_internet_gateway.igw.id}"

  timeouts {
    create = "15m"
  }
}
