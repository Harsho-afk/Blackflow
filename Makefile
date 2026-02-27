build:
	go build -o bin/blackflow ./cmd/blackflow

run:
	go run ./cmd/blackflow $(filter-out $@,$(MAKECMDGOALS))

clean:
	rm -rf bin

%:
	@:
