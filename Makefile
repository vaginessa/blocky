.PHONY: all clean build test e2e-test lint run fmt docker-build help
.DEFAULT_GOAL:=help

VERSION?=$(shell git describe --always --tags)
BUILD_TIME?=$(shell date '+%Y%m%d-%H%M%S')
DOCKER_IMAGE_NAME=spx01/blocky

BINARY_NAME:=blocky
BIN_OUT_DIR?=bin

GOARCH?=$(shell go env GOARCH)
GOARM?=$(shell go env GOARM)

GO_BUILD_FLAGS?=-v
GO_BUILD_LD_FLAGS:=\
	-w \
	-s \
	-X github.com/0xERR0R/blocky/util.Version=${VERSION} \
	-X github.com/0xERR0R/blocky/util.BuildTime=${BUILD_TIME} \
	-X github.com/0xERR0R/blocky/util.Architecture=${GOARCH}${GOARM}

GO_BUILD_OUTPUT:=$(BIN_OUT_DIR)/$(BINARY_NAME)$(BINARY_SUFFIX)

export PATH=$(shell go env GOPATH)/bin:$(shell echo $$PATH)

all: build test lint ## Build binary (with tests)

clean: ## cleans output directory
	rm -rf $(BIN_OUT_DIR)/*

serve_docs: ## serves online docs
	pip install mkdocs-material
	mkdocs serve

build:  ## Build binary
ifdef GO_SKIP_GENERATE
	$(info skipping go generate)
else
	go generate ./...
endif
	go build $(GO_BUILD_FLAGS) -ldflags="$(GO_BUILD_LD_FLAGS)" -o $(GO_BUILD_OUTPUT)
ifdef BIN_USER
	$(info setting owner of $(GO_BUILD_OUTPUT) to $(BIN_USER))
	chown $(BIN_USER) $(GO_BUILD_OUTPUT)
endif
ifdef BIN_AUTOCAB
	$(info setting cap_net_bind_service to $(GO_BUILD_OUTPUT))
	setcap 'cap_net_bind_service=+ep' $(GO_BUILD_OUTPUT)
endif

test: ## run tests
	go run github.com/onsi/ginkgo/v2/ginkgo --label-filter="!e2e" --coverprofile=coverage.txt --covermode=atomic -cover ./...

e2e-test: ## run e2e tests
	docker buildx build \
		--build-arg VERSION=blocky-e2e \
		--network=host \
		-o type=docker \
		-t blocky-e2e \
		.
	go run github.com/onsi/ginkgo/v2/ginkgo --label-filter="e2e" ./...

race: ## run tests with race detector
	go run github.com/onsi/ginkgo/v2/ginkgo --label-filter="!e2e" --race ./...

lint: ## run golangcli-lint checks
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1
	golangci-lint run --timeout 5m

run: build ## Build and run binary
	./$(BIN_OUT_DIR)/$(BINARY_NAME)

fmt: ## gofmt and goimports all go files
	find . -name '*.go' | while read -r file; do gofmt -w -s "$$file"; goimports -w "$$file"; done

docker-build:  ## Build docker image 
	go generate ./...
	docker buildx build \
		--build-arg VERSION=${VERSION} \
		--build-arg BUILD_TIME=${BUILD_TIME} \
		--network=host \
		-o type=docker \
		-t ${DOCKER_IMAGE_NAME} \
		.

help:  ## Shows help
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
