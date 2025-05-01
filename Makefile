.PHONY: build run demo

build:
	go build -o a2a-go main/main.go

run:
	go run ./a2a-go serve

demo:
	docker build -t theapemachine/a2a-go:latest .
	docker compose down
	docker compose up --build
