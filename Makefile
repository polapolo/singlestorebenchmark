run:
	@go mod vendor
	@go run main.go

docker:
	@sudo docker-compose up