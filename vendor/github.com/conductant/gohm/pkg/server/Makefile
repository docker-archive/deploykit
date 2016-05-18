all: test-server

test-server:
	${GODEP} go test ./...  -logtostderr -check.vv -v ${TEST_ARGS}

test-reverse-proxy:
	${GODEP} go test ./...  -logtostderr -check.vv -v -check.f TestSuiteReverseProxy ${TEST_ARGS}
