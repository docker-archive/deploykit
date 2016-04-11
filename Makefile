all: test-api

test-api:
	${GODEP} go test ./...  -check.vv -v ${TEST_ARGS}
