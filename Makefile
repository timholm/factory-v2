.PHONY: build test clean install

BINARY := factory-v2

build:
	go build -o bin/$(BINARY) .

test:
	go test ./... -v -count=1

clean:
	rm -rf bin/

install: build
	cp bin/$(BINARY) /usr/local/bin/$(BINARY)

lint:
	go vet ./...
