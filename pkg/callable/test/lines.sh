#!/bin/bash
{{/* The directive here tells infrakit to run this script with sh:  =% sh "-s" "--"  %=  */}}
{{ $lines := flag "lines" "int" "the number of lines" 5 }}
{{ $header := param "header" "string" "the header" "default" }}

for i in $(seq {{$lines}}); do
echo {{ $header }} $i
done
