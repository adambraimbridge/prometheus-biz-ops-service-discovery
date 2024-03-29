SHELL := /bin/bash

TEAL = $(shell printf '%b' "\033[0;36m")
GREEN = $(shell printf '%b' "\033[0;32m")
RED = $(shell printf '%b' "\033[0;31m")
NO_COLOUR = $(shell printf '%b' "\033[m")

GOBIN = $(shell pwd)/bin
PATH=$$GOBIN:$(shell echo $$PATH)
PACKAGES = $(shell go list ./... | grep -v /vendor/)
UPPER_CASE_REPO_NAME = $(shell $(	REPO_NAME) | sed -r 's/\<./\U&/g')
AWS := $(shell command aws --version 2> /dev/null)
DONE = printf '%b\n' ">> $(GREEN)$@ done ✓"

DOCKER_TEAM_NAME ?= operations-reliability
DOCKER_TAG ?= latest
PORT ?= 8080

ifneq ("$(CIRCLE_SHA1)", "")
VCS_SHA := $(CIRCLE_SHA1)
else
VCS_SHA = $(shell git rev-parse HEAD)
endif

ifneq ("$(CIRCLE_BUILD_NUM)", "")
BUILD_NUMBER := $(CIRCLE_BUILD_NUM)
else
BUILD_NUMBER := n/a
endif

ifneq ("$(CIRCLE_PROJECT_REPONAME)", "")
REPO_NAME := $(CIRCLE_PROJECT_REPONAME)
else
REPO_NAME = $(shell basename `git rev-parse --show-toplevel`)
endif

all: format build test

test: ## Run the tests 🚀.
	@printf '%b\n' ">> $(TEAL)running tests"
	go test -short $(PACKAGES)
	@$(DONE)

test-report: ## Run the tests and get junit output.
	@printf '%b\n' ">> $(TEAL)running tests"
	go get github.com/jstemmer/go-junit-report
	mkdir -p ./test-results/go-tests
	go test -v $(PACKAGES) | go-junit-report -set-exit-code > ./test-results/go-tests/go-test-report.xml
	@$(DONE)

style: ## Check the formatting of the Go source code.
	@printf '%b\n' ">> $(TEAL)checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'
	@$(DONE)

format: ## Format the Go source code.
	@printf '%b\n' ">> $(TEAL)formatting code"
	go fmt $(PACKAGES)
	@$(DONE)

vet: ## Examine the Go source code.
	@printf '%b\n' ">> $(TEAL)vetting code"
	go vet $(PACKAGES)
	@$(DONE)

build: ## Build the Docker image.
	@printf '%b\n' ">> $(TEAL)building the docker image"
	docker build \
		-t "financial-times/$(REPO_NAME):$(VCS_SHA)" \
		--build-arg BUILD_DATE="$(shell date '+%FT%T.%N%:z')" \
		--build-arg VCS_SHA=$(VCS_SHA) \
		--build-arg BUILD_NUMBER=$(BUILD_NUMBER) \
		.
	@$(DONE)

run: ## Run the Docker image.
	@printf '%b\n' ">> $(TEAL)running the docker image"
	docker run \
		-e DIRECTORY=out \
		-e "BIZ_OPS_API_KEY=$(BIZ_OPS_API_KEY)" \
		-e "PORT=$(PORT)" \
		-v $(PWD)/out:/root/out \
		"financial-times/$(REPO_NAME):$(VCS_SHA)" $(ARGS)
	@$(DONE)

publish: ## Push the docker image to the FT private repository.
	@printf '%b\n' ">> $(TEAL)pushing the docker image"
	docker tag "financial-times/$(REPO_NAME):$(VCS_SHA)" "nexus.in.ft.com:5000/$(DOCKER_TEAM_NAME)/$(REPO_NAME):$(DOCKER_TAG)"
	docker push "nexus.in.ft.com:5000/$(DOCKER_TEAM_NAME)/$(REPO_NAME):$(DOCKER_TAG)"
	@$(DONE)

validate-aws-stack-command:
	@if [[ -z "$(AWS)" ]]; then echo "❌ $(RED)AWS is not available please install aws-cli. See https://aws.amazon.com/cli/" && exit 1; fi
	@if [[ -z "$(ECS_CLUSTER_NAME)" ]]; then echo "❌ $(RED)ECS_CLUSTER_NAME is not available. Please specify the ECS cluster to deploy to" && exit 1; fi
	@if [[ -z "$(SPLUNK_HEC_TOKEN)" ]]; then echo "❌ $(RED)SPLUNK_HEC_TOKEN is not available. $(NO_COLOUR)This is a required variable for cloudformation deployments" && exit 1; fi
	@if [[ -z "$(BIZ_OPS_API_KEY)" ]]; then echo "❌ $(RED)BIZ_OPS_API_KEY is not available. $(NO_COLOUR)This is a required variable for cloudformation deployments" && exit 1; fi

deploy-stack: validate-aws-stack-command ## Create the cloudformation stack
	@printf '%b\n' ">> $(TEAL)deploying cloudformation stack"
	@aws cloudformation deploy \
		--stack-name "$(ECS_CLUSTER_NAME)-service-$(REPO_NAME)" \
		--template-file deployments/cloudformation.yml \
		--parameter-overrides \
			ParentClusterStackName=$(ECS_CLUSTER_NAME) \
			SplunkHecToken=$(SPLUNK_HEC_TOKEN) \
			BizOpsApiKey=$(BIZ_OPS_API_KEY) \
			DockerRevision="$(DOCKER_TAG)" \
		--role-arn "arn:aws:iam::442980623726:role/FTDeployRoleFor_mon-agg-ecs-cfn" \
		--no-fail-on-empty-changeset \
		--tags \
        	environment="p" \
        	systemCode="$(REPO_NAME)" \
        	teamDL="reliability.engineering@ft.com"
	@$(DONE)

help: ## Show this help message.
	@printf '%b\n' "usage: make [target] ..."
	@printf '%b\n' ""
	@printf '%b\n' "targets:"
	@# replace the first : with £ to avoid splitting columns on URLs
	@grep -Eh '^[^_].+?:\ ##\ .+' ${MAKEFILE_LIST} | cut -d ' ' -f '1 3-' | sed 's/^(.+?):/$1/' | sed 's/:/£/' | column -t -c 2 -s '£'

.PHONY: all style format build test vet deploy-stack
