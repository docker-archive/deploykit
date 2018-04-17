# IAM:

# Create an IAM role for the Web Servers.
resource "aws_iam_role" "provisioner_role" {
  path = "/"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "provisioner_policy" {
  name = "managers-policy"
  role = "${aws_iam_role.provisioner_role.id}"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "*",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_iam_instance_profile" "bootstrap" {
  role = "${aws_iam_role.provisioner_role.id}"
  path = "/"
}

# resource "aws_iam_role_policy" "ec2_role_policy" {
#   name = "${var.deployment}_ec2_role_policy"
#   role = "${aws_iam_role.ec2_role.id}"


#   policy = <<EOF
# {
#   "Version": "2012-10-17",
#   "Statement": [
#     {
#       "Effect": "Allow",
#       "Action": [
#         "ec2:CreateTags",
#         "ec2:DescribeTags",


#         "ec2:CreateSnapshot",
#         "ec2:DeleteSnapshot",
#         "ec2:DescribeSnapshots",


#         "ec2:CreateVolume",
#         "ec2:DeleteVolume",
#         "ec2:DescribeVolumes",
#         "ec2:AttachVolume",
#         "ec2:DetachVolume",


#         "logs:CreateLogStream",
#         "logs:PutLogEvents",
#         "cloudwatch:PutMetricData"
#       ],
#       "Resource": "*"
#     }
#   ]
# }
# EOF
# }

