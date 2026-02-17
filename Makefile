build:
	go build -o bin/blackflow ./cmd/blackflow

run:
	go run ./cmd/blackflow

clean:
	rm -rf bin
