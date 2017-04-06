#CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/imgur .
GOOS=linux go build -o bin/imgur .
