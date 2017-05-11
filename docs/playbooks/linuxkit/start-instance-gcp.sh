#!/bin/bash

{{/* =% sh %= */}}

{{ $gcp := flag "start-gcp" "bool" "Start GCP plugin" | prompt "Start GCP plugin?" "bool" "no" }}

{{ $defaultCred := cat (env "HOME") "/.config/gcloud/application_default_credentials.json" | nospace }}
{{ $credentials := flag "credential-path" "string" "Path to credentials.json" | cond $gcp | prompt "Credentials JSON path?" "string" $defaultCred }}
{{ $zone := flag "zone" "string" "GCP zone" | cond $gcp | prompt "What's the zone?" "string" "us-central1-f"}}

{{ $gcpImage := flag "gcp-plugin" "string" "Image of the plugin" |  cond $gcp | prompt "What's the GCP plugin image?" "string" "infrakit/gcp" }}

{{ $project := var "/project" }}

{{ if $gcp }}

echo "Starting GCP plugin"

{{ $gcpCredentials :=  (cat $credentials ":/infrakit/platforms/gcp/credentials.json" | nospace) }}


# Starting docker container for instance plugin
docker run -d --volumes-from infrakit --name instance-gcp \
       -v {{$gcpCredentials}} {{$gcpImage}} infrakit-instance-gcp  \
       --namespace-tags {{cat "infrakit.scope=" $project | nospace}} \
       --zone {{ $zone }} --log 5 --project {{ $project }}

{{ var `started-gcp` true }}

{{ end }}
