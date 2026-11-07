.PHONY: build test integration cover fuzz vet fmt check clean

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

# Runs the real binary against the real kernel; needs the AppArmor profile (scripts/install-apparmor.sh).
integration: build
	go test -count=1 -v ./test/integration

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Everything that must pass before a commit.
check: vet
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	go test -race ./...
	GOOS=linux GOARCH=arm64 go vet ./...
	GOOS=linux GOARCH=riscv64 go vet ./...

clean:
	rm -rf bin coverage.out
