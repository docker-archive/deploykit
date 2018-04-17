output "AppDNSTarget" {
  description = "Use this name to update your DNS records"
  value       = "${aws_elb.apps.dns_name}"
}

output "BootNodeIP" {
  description = "List of manager public IP"
  value       = "${aws_instance.bootstrap.public_ip}"
}
