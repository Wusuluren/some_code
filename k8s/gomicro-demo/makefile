setup:
	go install go-micro.dev/v4/cmd/micro@master
	go install go-micro.dev/v4/cmd/protoc-gen-micro@v4
	go mod init gomicro-demo

generate-time:
	micro new service time-service

run:
    cd time-service
    go run main.go &
    cd ..
    cd gateway-service
    go run main.go &
    cd ..
    curl  "127.0.0.1:8000/time/getTime?timezone=Local"
