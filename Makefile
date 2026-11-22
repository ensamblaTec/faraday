BIN := faraday
export CGO_ENABLED := 0

.PHONY: build test install clean
build:
	go build -o $(BIN) ./cmd/faraday
test:
	go test ./...
install: build
	install -Dm755 $(BIN) $(HOME)/.local/bin/$(BIN)
clean:
	rm -f $(BIN)
