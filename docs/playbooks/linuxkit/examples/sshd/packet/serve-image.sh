{{/* =% sh %= */}}

{{ $image := `infrakit/devbundle:dev` }}
{{ $path := flag `path` `string` `Path to serve from` | prompt "Directory to serve kernel images from?" `string` (env `PWD`) }}
{{ $port := flag `port` `int` `Listen port` | prompt "Listening port?" `int` 8080 }}

echo "Starting up file server from {{$path}}"

docker run  -d --rm --name infrakit-fileserver \
       -v {{ $path }}:/files -p {{ $port }}:8080 \
       {{ $image }} infrakit util fileserver /files


echo "Don't forget to start up ngrok!!!"
