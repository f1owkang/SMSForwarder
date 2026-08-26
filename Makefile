VERSION ?= dev
ARCHES  = amd64 arm64 arm armv6
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
		case "$$arch" in \
		  amd64) goarch=amd64; goarm=;; \
		  arm64) goarch=arm64; goarm=;; \
		  arm)   goarch=arm;   goarm=7;; \
		  armv6) goarch=arm;   goarm=6;; \
		esac; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$goarch GOARM=$$goarm \
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
