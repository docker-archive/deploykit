.PHONY: clean container

.DEFAULT_GOAL = container

.CONTAINER_OWNER:=

clean:
	@echo "+ $@"
	rm -rf build
	mkdir -p build

container: clean
ifndef CONTAINER_OWNER
    $(error please specify CONTAINER_OWNER env variable)
endif

	@echo "+ $@"
	docker build -t infrakit-plugins-build -f Dockerfile.build .
	docker run --rm \
		-v ${CURDIR}/build:/build \
		infrakit-plugins-build
	docker build -t ${CONTAINER_OWNER}/infrakit-demo-plugins .
