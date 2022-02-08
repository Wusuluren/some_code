setup:
    go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
    go mod init kitex-demo

generate-time:
	cd time-service
	kitex -type protobuf -module kitex-demo -service timer-service -I ../pb ../pb/time.proto

run:
    cd time-service
    ./build
    ./output/bootstrap.sh &
    cd ..
    go run gateway-service/main.go
    curl  "127.0.0.1:8000/time/getTime?timezone=Local"
