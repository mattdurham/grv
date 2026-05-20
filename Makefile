.PHONY: install build test test-race test-cover clean

install: build
	install -m 755 grv /usr/local/bin/grv

build:
	go build -o grv .

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -f grv coverage.out
