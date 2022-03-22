run:
	@go mod vendor
	@go run main.go

docker:
	@sudo docker-compose up

mod:
	@go mod tidy
	@go mod vendor