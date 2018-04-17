# ELBs:

resource "aws_elb" "apps" {
  name = "${var.deployment}-apps"

  security_groups = [
    "${aws_security_group.instances.id}",
  ]

  subnets = ["${aws_subnet.pubsubnet.*.id}"]

  listener {
    instance_port     = 7
    instance_protocol = "tcp"
    lb_port           = 7
    lb_protocol       = "tcp"
  }

  health_check {
    healthy_threshold   = 2
    unhealthy_threshold = 10
    timeout             = 5
    target              = "TCP:22"
    interval            = 30
  }

  instances                 = ["${aws_instance.bootstrap.id}"]
  cross_zone_load_balancing = true
  depends_on                = ["aws_internet_gateway.igw"]
}
