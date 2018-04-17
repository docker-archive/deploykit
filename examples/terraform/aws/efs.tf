resource "aws_efs_file_system" "cloudstor-gp" {
  count = "${var.efs_supported}"

  tags {
    Name = "${format("%s-Cloudstor-GP", var.deployment)}"
  }

  performance_mode = "generalPurpose"
}

resource "aws_efs_file_system" "cloudstor-maxio" {
  count = "${var.efs_supported}"

  tags {
    Name = "${format("%s-Cloudstor-MaxIO", var.deployment)}"
  }

  performance_mode = "maxIO"
}

resource "aws_efs_mount_target" "cloudstor-gp" {
  file_system_id = "${aws_efs_file_system.cloudstor-gp.id}"
  count          = "${var.efs_supported * length(data.aws_availability_zones.available.names)}"
  subnet_id      = "${element(aws_subnet.pubsubnet.*.id, count.index)}"

  # security_groups = ["${aws_security_group.managers.id}", "${aws_security_group.workers.id}"]
}

resource "aws_efs_mount_target" "cloudstor-maxio" {
  file_system_id = "${aws_efs_file_system.cloudstor-maxio.id}"
  count          = "${var.efs_supported * length(data.aws_availability_zones.available.names)}"
  subnet_id      = "${element(aws_subnet.pubsubnet.*.id, count.index)}"

  # security_groups = ["${aws_security_group.managers.id}", "${aws_security_group.workers.id}"]
}
