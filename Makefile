BINARY := lazydc
PKG := ./cmd/lazydc

.PHONY: build test fmt vet tidy run clean

build:
	go build -o $(BINARY) $(PKG)

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

vet:
	go vet ./...

tidy:
	go mod tidy

run:
	go run $(PKG)

clean:
	rm -f $(BINARY)
