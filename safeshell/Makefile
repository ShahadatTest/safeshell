.PHONY: build test cover fuzz vet fmt check clean

build:
	go build -o bin/ss ./cmd/ss

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

# Short fuzz runs of the parser and classifier; raise FUZZTIME for longer.
FUZZTIME ?= 20s
fuzz:
	go test ./internal/shellparse -run xxx -fuzz FuzzParse -fuzztime $(FUZZTIME)
	go test ./internal/classify -run xxx -fuzz FuzzClassify -fuzztime $(FUZZTIME)

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Everything that must pass before a commit.
check: vet test
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

clean:
	rm -rf bin coverage.out
