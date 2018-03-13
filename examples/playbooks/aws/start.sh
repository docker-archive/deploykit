{{/* =% sh %= */}}

{{ $clearState := flag "clear-state" "bool" "Clear stored states" | prompt "Clear state?" "bool" true }}

{{ $profile := flag "aws-cred-profile" "string" "Profile name" | prompt "Profile for your .aws/credentials?" "string" "default" }}
{{ $region := flag "region" "string" "aws region" | prompt "Region?" "string" "eu-central-1"}}
{{ $project := flag "project" "string" "project name" | prompt "Project?" "string" "myproject" }}

{{ if $clearState }}
rm -rf ~/.infrakit/plugins/* # remove sockets, pid files, etc.
rm -rf ~/.infrakit/configs/global.config # for file based manager
# Since we are using file based leader detection, write the default name (manager1) to the leader file.
echo manager1 > ~/.infrakit/leader
{{ end }}

echo "Starting infrakit with aws plugin..."

{{/* Pick a credential from the local ~/.aws folder.  You should have this if you use awscli. */}}
{{ $creds := index (source (cat "file://" (env "HOME") "/.aws/credentials" | nospace) | iniDecode ) $profile }}
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
infrakit plugin start manager:mystack vars group resource aws \
	 --log 5 --log-stack --log-debug-V 500 \
	 --log-debug-match module=controller/resource \
	 --log-debug-match module=controller/internal \
	 --log-debug-match module=provider/aws \
