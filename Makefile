.PHONY: build run test demo server client logs

build:
	go build -o a2a-go main/main.go

run:
	go run ./a2a-go serve

test:
	docker build -t theapemachine/a2a-go:latest .
	docker compose down catalog dockertool browsertool ui manager planner researcher developer
	docker compose up --build --remove-orphans --force-recreate catalog dockertool browsertool ui manager planner researcher developer

demo:
	docker build -t theapemachine/a2a-go:latest .
	docker compose down catalog dockertool browsertool ui manager planner researcher developer
	docker compose up --build -d --remove-orphans --force-recreate catalog dockertool browsertool ui manager planner researcher developer
	docker run -it --rm \
		--name a2a-go-client \
		--network a2a-go_a2a-network \
		-e CATALOG_URL=http://catalog:3210 \
		-e SSE_RETRY_INTERVAL=3000 \
		-e SSE_MAX_RETRIES=10 \
		theapemachine/a2a-go:latest ui

server:
	docker build -t theapemachine/a2a-go:latest .
	docker compose down
	docker compose up --build -d --remove-orphans --force-recreate
	docker compose logs -f catalog dockertool browsertool ui manager planner researcher developer
	
client:
	docker run -it --rm \
		--name a2a-go-client \
		--network a2a-go_a2a-network \
		-e CATALOG_URL=http://catalog:3210 \
		-e SSE_RETRY_INTERVAL=3000 \
		-e SSE_MAX_RETRIES=10 \
		theapemachine/a2a-go:latest ui

logs:
	docker compose logs -f catalog dockertool browsertool ui manager planner researcher developer