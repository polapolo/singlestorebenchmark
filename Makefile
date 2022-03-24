build:
	@go build -o main .

run:
	@make build
	@./main

docker:
	@sudo docker-compose up

mod:
	@go mod vendor