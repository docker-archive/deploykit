# ---------------------------------------------------------------------------------------------------------------------
# CREATE ALL INSTANCES FOR MANAGERS AND WORKERS
# ---------------------------------------------------------------------------------------------------------------------

data "template_file" "user_data" {
  template = <<EOD
#!/usr/bin/env bash
########################
# Install Docker
########################

curl -fsSL get.docker.com -o - | bash
usermod -aG docker $${linux_user}

########################
# Setup Infrakit
########################
mkdir -p /infrakit
docker swarm init
docker run -v /var/run/docker.sock:/var/run/docker.sock -v /infrakit:/infrakit $${infrakit_image} \
INFRAKIT_MANAGER_BACKEND=swarm \
infrakit util init --group-id managers --start combo --start swarm --wait 5s \
 --var /cluster/provider=aws \
 --var /cluster/name=$${deployment} \
 --var /cluster/size=$${cluster_size} \
 --var /infrakit/config/root=$${config_root} \
 --var /infrakit/metadata/configURL=$${metadata_url} \
 --var /provider/image/hasDocker=yes \
 --var /infrakit/docker/image=$${infrakit_image} \
 $${config_root}/groups.json | tee /var/lib/infrakit.boot | sh
EOD

  vars {
    infrakit_image = "${var.infrakit}"
    linux_user     = "${var.linux_user}"
    deployment     = "${var.deployment}"
    config_root    = "${var.config_root}"
    cluster_size   = "${var.cluster_size}"
    metadata_url   = "${var.metadata}"
  }
}

################################
#
# Bootstrap Node
#

resource "aws_instance" "bootstrap" {
  ami                  = "${data.aws_ami.linux_ami.image_id}"
  instance_type        = "${var.instance_type}"
  key_name             = "${var.key_name}"
  iam_instance_profile = "${aws_iam_instance_profile.bootstrap.id}"
  subnet_id            = "${local.bootstrap_subnet_id}"
  private_ip           = "${local.bootstrap_ip}"

  # Second disk for docker storage
  ebs_block_device {
    device_name           = "/dev/xvdb"
    volume_size           = "${var.volume_size}"
    volume_type           = "gp2"
    delete_on_termination = true
  }

  volume_tags {
    Name                   = "${format("%s-ebs", var.deployment)}"
    infrakit.scope         = "${var.deployment}"
    docker-infrakit-volume = "${local.bootstrap_ip}"
  }

  vpc_security_group_ids = ["${aws_security_group.instances.id}"]
  user_data              = "${data.template_file.user_data.rendered}"

  tags {
    Name                = "${format("%s-Bootstrap", var.deployment)}"
    infrakit.scope      = "${var.deployment}"
    infrakit.group      = "managers"
    infrakit.config_sha = "bootstrap"
    infrakit.role       = "managers"
  }
}
