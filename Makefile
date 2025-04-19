.PHONY: build

build:
	go build -o a2a-go main/main.go

run:
	go run ./a2a-go serve

