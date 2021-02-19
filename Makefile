DOCKER_PUSH_REPOSITORY = something
APP_NAME = metris
DOCKER_TAG = dev

.PHONY: clean
clean:
	rm -rf bin
	rm -rf vendor

# Default configuration
IMG_NAME := $(DOCKER_PUSH_REPOSITORY)/$(APP_NAME)
TAG := $(DOCKER_TAG)
IMG_GOPATH := /go
BASE_PKG := github.com/kyma-incubator/metris

# Other variables
# COMPONENT_DIR is a local path to component
COMPONENT_DIR = $(shell pwd)
# FILES_TO_CHECK is a command used to determine which files should be verified
FILES_TO_CHECK = find . -type f -name "*.go" | grep -v "$(VERIFY_IGNORE)"
# VERIFY_IGNORE is a list of all folders which are generated and should be ignored
VERIFY_IGNORE := /vendor
# DIRS_TO_CHECK is a command used to determine which directories should be verified
DIRS_TO_CHECK = go list ./... | grep -v "$(VERIFY_IGNORE)"

ifndef ARTIFACTS
ARTIFACTS:=/tmp/artifacts
endif

.PHONY: verify format
verify:: test check-imports check-fmt
format:: imports fmt

.PHONY: build-image push-image
build-image: gomod-deps
	docker build -t $(IMG_NAME) .

push-image:
	docker tag $(IMG_NAME) $(IMG_NAME):$(TAG)
	docker push $(IMG_NAME):$(TAG)

gomod-deps: gomod-vendor gomod-verify gomod-status

gomod-check: test check-imports check-fmt

gomod-component-check: gomod-deps gomod-check

gomod-release: gomod-component-check build-image push-image

gomod-vendor:
	GO111MODULE=on go mod vendor -v

gomod-verify:
	GO111MODULE=on go mod verify

gomod-status:
	GO111MODULE=on go mod graph

gomod-tidy:
	GO111MODULE=on go mod tidy

## Source Code tools
check-imports:
	@if [ -n "$$(goimports -l $$($(FILES_TO_CHECK)))" ]; then \
		echo "✗ some files contain not propery formatted imports. To repair run make imports"; \
		goimports -l $$($(FILES_TO_CHECK)); \
		exit 1; \
	fi;

imports:
	goimports -w -l $$($(FILES_TO_CHECK))

check-fmt:
	@if [ -n "$$(gofmt -l $$($(FILES_TO_CHECK)))" ]; then \
		gofmt -l $$($(FILES_TO_CHECK)); \
		echo "✗ some files contain not propery formatted imports. To repair run make imports-local"; \
		exit 1; \
	fi;

fmt:
	go fmt $$($(DIRS_TO_CHECK))

errcheck:
	errcheck -blank -asserts -verbose  -ignorepkg '$(go list ./... | grep -v "/vendor" | tr '\n' ',')'  -exclude errcheck_excludes.txt  -excludeonly ./...

vet:
	go vet $$($(DIRS_TO_CHECK))

tests:
	go test -cover ./...

publish-test-results:
	go test -coverprofile cover.out ./...
	go tool cover -html=cover.out

docker-compose-up:
	docker-compose up

.PHONY: list
list:
	@$(MAKE) -pRrq -f $(COMPONENT_DIR)/Makefile : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$'
