.PHONY: build test vet run tidy smoke tarball

APP=diary-relay
CONFIG ?= config.yaml

build:
	go build -o bin/$(APP) ./cmd/server

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

check: vet test

run: build
	./bin/$(APP) -config $(CONFIG)

smoke:
	bash scripts/smoke_test.sh

tarball:
	tar -C /workspace -czf /workspace/diary-relay.tgz \
		--exclude='diary-relay/bin' \
		--exclude='diary-relay/data' \
		--exclude='diary-relay/preview' \
		--exclude='diary-relay/smoke-*' \
		--exclude='diary-relay/smoke-run' \
		--exclude='diary-relay/.diary-relay-*' \
		diary-relay
	@ls -lh /workspace/diary-relay.tgz
