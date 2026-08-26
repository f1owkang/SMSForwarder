VERSION ?= dev
ARCHES  = amd64 arm64 arm
LDFLAGS = -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := test

.PHONY: test vet fmt build-all package clean

fmt:
	gofmt -l -w .

vet:
	go vet ./...

test:
	go test ./...

build-all:
	@for arch in $(ARCHES); do \
		mkdir -p dist/smsforwarder-linux-$$arch; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch GOARM=7 \
		go build -trimpath -ldflags "$(LDFLAGS)" \
		-o dist/smsforwarder-linux-$$arch/smsforwarder ./cmd/smsforwarder; \
	done

package: build-all
	@for arch in $(ARCHES); do \
		cp config.example.yml stopwords.txt userwords.txt smsforwarder.service dist/smsforwarder-linux-$$arch/; \
		tar czf dist/smsforwarder-$(VERSION)-linux-$$arch.tar.gz -C dist/smsforwarder-linux-$$arch .; \
	done

clean:
	rm -rf dist
