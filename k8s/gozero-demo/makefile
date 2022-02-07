setup:
	GO111MODULE=on GOPROXY=https://goproxy.cn/,direct go get -u github.com/zeromicro/go-zero/tools/goctl
	go mod init gozero-demo

generate:
	rm -rf gateway-service || true
	goctl api go -api gateway.api -dir gateway-service
	rm -rf time-service pb || true
	cd protobuf; goctl rpc protoc time.proto --go_out=../ --go-grpc_out=../ --zrpc_out=../time-service;cd ..
	go mod tidy