{{/* =% sh %= */}}

{{ $profile := flag "aws-cred-profile" "string" "Profile name" | prompt "Profile for your .aws/credentials?" "string" "default" }}
{{ $region := flag "region" "string" "aws region" | prompt "Region?" "string" "eu-central-1"}}
{{ $project := flag "project" "string" "project name" | prompt "Project?" "string" "myproject" }}

echo "Starting infrakit with aws plugin..."

{{/* Pick a credential from the local user's ~/.aws folder.  You should have this if you use awscli. */}}
{{ $creds := (source (cat "file://" (env "HOME") "/.aws/credentials" | nospace) | iniDecode | k $profile ) }}
FOUND="{{ not (empty $creds) }}"

if [ $FOUND = "false" ]; then
  echo "no credentials found. bye"
  exit 1
fi

{{ echo "Found your credential for profile" $profile }}

AWS_ACCESS_KEY_ID={{ $creds.aws_access_key_id }} \
AWS_SECRET_ACCESS_KEY={{ $creds.aws_secret_access_key }} \
INFRAKIT_AWS_REGION={{ $region }} \
INFRAKIT_AWS_STACK_NAME={{ $project }} \
INFRAKIT_AWS_NAMESPACE_TAGS="infrakit.scope={{ $project }}" \
INFRAKIT_AWS_MONITOR_POLL_INTERVAL=5s \
infrakit plugin start aws
