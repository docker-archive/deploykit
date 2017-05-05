.PHONY: test
WORKSPACE = $(shell pwd)

topdir = /tmp/$(pkg)-$(version)

all: container runcontainer
	@true

container:
	docker build --no-cache -t builder-stow test/

runcontainer:
	docker run -v $(WORKSPACE):/mnt/src/github.com/graymeta/stow builder-stow

deps:
	go get github.com/tebeka/go2xunit
	go get github.com/Azure/azure-sdk-for-go/storage
	go get github.com/aws/aws-sdk-go
	go get github.com/ncw/swift
	go get github.com/cheekybits/is
	go get golang.org/x/net/context
	go get golang.org/x/oauth2/google
	go get github.com/pkg/errors
	go get google.golang.org/api/storage/...

test: clean deps vet
	go test -v ./... | tee tests.out
	go2xunit -fail -input tests.out -output tests.xml

vet:
	go vet ./...

clean:
	rm -f tests.out test.xml
