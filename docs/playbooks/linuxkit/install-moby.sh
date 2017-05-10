{{/* =% sh %= */}}

{{ $goos := flag "os" "string" "GOOS value" | prompt "GOOS?" "string" "darwin" }}
{{ $goarch := flag "arch" "string" "GOARCH value" | prompt "GOARCH?" "string" "amd64" }}

{{ $mobyCompile := flag "moby-compile" "string" "Moby compiler container" | prompt "Moby go-compile container?" "string" "linuxkit/go-compile:5bf17af781df44f07906099402680b9a661f999b" }}
{{ $mobyCommit := flag "moby-commit" "string" "Moby commit hash" | prompt "Moby commit SHA?" "string" "d504afe4795528920ef06af611efd27b74098d5e" }}

echo "Installing Moby build tool on your system."

export CROSS=""

docker run --rm --log-driver=none -e GOOS={{$goos}} -e GOARCH={{$goarch}} {{$mobyCompile}} \
       --clone-path github.com/moby/tool \
       --clone https://github.com/moby/tool.git \
       --commit {{$mobyCommit}} \
       --package github.com/moby/tool/cmd/moby \
       --ldflags "-X main.GitCommit={{$mobyCommit}} -X main.Version={{$mobyCommit}}" \
       -o moby > tmp_moby_bin.tar

tar xf tmp_moby_bin.tar
rm tmp_moby_bin.tar

echo "Built moby:"
`pwd`/moby -h

echo "Copying moby to /usr/local/bin"
sudo cp `pwd`/moby /usr/local/bin/
