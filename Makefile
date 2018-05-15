PACKAGES = $(shell go list ./... | grep -v /vendor/)
DONE = echo ">> $@ done"

DOCKER_TEAM_NAME 	?= operations-reliability
DOCKER_IMAGE_NAME   ?= prometheus-biz-ops-service-discovery

all: format build test

test: ## Run the tests 🚀.
	@echo ">> running tests"
	go test -short $(PACKAGES)
	@$(DONE)

style: ## Check the formatting of the Go source code.
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'
	@$(DONE)

format: ## Format the Go source code.
	@echo ">> formatting code"
	go fmt $(PACKAGES)
	@$(DONE)

vet: ## Examine the Go source code.
	@echo ">> vetting code"
	go vet $(PACKAGES)
	@$(DONE)

build: ## Build the binary.
	@echo ">> building the binary"
	go build -o biz-ops-service-discovery cmd/biz-ops-service-discovery/main.go
	@$(DONE)

docker: ## Build the docker image.
	@echo ">> building the docker image"
	docker build -t "$(DOCKER_IMAGE_NAME):latest" .
	@$(DONE)

docker-push: ## Push the docker image to the FT private repository.
docker-push: docker
	@echo ">> pushing the docker image"
	docker tag "$(DOCKER_IMAGE_NAME):latest" "nexus.in.ft.com:5000/$(DOCKER_TEAM_NAME)/$(DOCKER_IMAGE_NAME):latest"
	docker push "nexus.in.ft.com:5000/$(DOCKER_TEAM_NAME)/$(DOCKER_IMAGE_NAME):latest"
	@$(DONE)

help: ## Show this help message.
	@echo "usage: make [target] ..."
	@echo ""
	@echo "targets:"
	@grep -Eh '^.+:\ ##\ .+' ${MAKEFILE_LIST} | column -t -s ':#'

.PHONY: all style format build test vet docker
