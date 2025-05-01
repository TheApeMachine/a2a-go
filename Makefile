.PHONY: build run demo

build:
	go build -o a2a-go main/main.go

run:
	go run ./a2a-go serve

demo:
    echo "🏗️ Rebuilding the docker image"
	docker build -t theapemachine/a2a-go:latest .
	
	echo "📥 Stopping any running A2A containers"
	docker compose down

	echo "🚀 Starting A2A services"
	docker compose up --build

	echo "⏳ Waiting for catalog service to be healthy..."
	for i in {1..12}; do
		HEALTH=$(docker inspect --format='{{.State.Health.Status}}' $(docker-compose ps -q catalog) 2>/dev/null || echo "not found")
		
		if [ "$HEALTH" == "healthy" ]; then
			echo "✅ Catalog service is healthy!"
			break
		fi
		
		if [ "$HEALTH" == "not found" ]; then
			echo "❌ Catalog container not found! Something went wrong."
			docker-compose logs catalog
			exit 1
		fi
		
		if [ $i -eq 12 ]; then
			echo "❌ Timed out waiting for catalog service to become healthy."
			echo "🔍 Checking catalog logs..."
			docker-compose logs catalog
			exit 1
		fi
		
		echo "⏳ Catalog status: $HEALTH (attempt $i/12)..."
		sleep 5
	done
