{{/* =% sh %= */}}

{{ $defaultKeyDir := cat (env `HOME`) `/.ssh` | nospace }}

{{ $keyDir := flag "ssh-dir" "string" "Directory of ssh keys" | prompt "SSH key dir?" "string" $defaultKeyDir }}
{{ $keyFile := flag "key-name" "string" "Key name" | prompt "Which public key to include?" "string" "id_rsa.pub" }}

{{ $output := `sshd.yml` }}

{{/* load the file as the public key */}}
{{/* $keyFile := cat $keyDir `/` $keyFile | nospace */}}

{{ $keyFile := list `file://` $keyDir $keyFile | join `/` }}

echo "Reading from {{ $keyFile }}"

{{ include $keyFile | var `public_key` }}

infrakit template -o {{$output}} - <<EOF
{{ source "sshd.yml" }}
EOF

echo "Generated config file. Running moby."

moby build {{$output}}

{{ $bundle := `sshd` }}
echo "Checksum:"{{ fetch (cat `file://` (env `PWD`) $bundle "-initrd.img" | nospace) | sha256sum }}
