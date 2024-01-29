up:
	docker compose up -d --build --remove-orphans

upnb:
	docker compose up -d --remove-orphans

down:
	docker compose down

shell:
	docker compose exec dev sh

run:
	go run main.go

debug:
	dlv debug --headless --listen=:40000 --api-version=2 --accept-multiclient

test:
	go test -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
