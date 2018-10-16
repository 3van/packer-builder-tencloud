TEST?=$(shell go list ./... | grep -v vendor)
VET?=$(shell ls -d */ | grep -v vendor | grep -v website)
UNFORMATTED_FILES=$(shell find . -not -path "./vendor/*" -name "*.go" | xargs gofmt -s -l)

.PHONY: build
build:
	go build -v

.PHONY: install
install:
	go install -v

.PHONY: fmt
fmt:
	@gofmt -w -s main.go $(UNFORMATTED_FILES)

.PHONY: fmt-check
fmt-check:
	@echo "==> Checking that code complies with gofmt requirements..."
	@if [ ! -z "$(UNFORMATTED_FILES)" ]; then \
		echo "gofmt needs to be run on the following files:"; \
		echo "$(UNFORMATTED_FILES)" | xargs -n1; \
		echo "You can use the command: \`make fmt\` to reformat code."; \
		exit 1; \
	else \
		echo "Check passed."; \
	fi

.PHONY: test
test: fmt-check
	@go test $(TEST) $(TESTARGS) -timeout=2m
	@go tool vet $(VET)  ; if [ $$? -eq 1 ]; then \
		echo "ERROR: Vet found problems in the code."; \
		exit 1; \
	fi
