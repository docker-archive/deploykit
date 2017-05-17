{{/* =% sh %= */}}

{{ $plugin := flag "instance-plugin" "string" "Name of plugin" | prompt "What instance plugin to target?" "string" "instance-packet" }}

infrakit {{ $plugin }} describe -q | awk '{print $1}' | xargs infrakit instance --name {{ $plugin }} destroy
