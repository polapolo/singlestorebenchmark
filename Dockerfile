FROM golang:1.17.8
WORKDIR /singlestorebenchmark
COPY . /singlestorebenchmark

# Install dependencies
RUN go mod vendor -v

# Compile
RUN go build -o main .

# Run
ENTRYPOINT "/singlestorebenchmark/main"