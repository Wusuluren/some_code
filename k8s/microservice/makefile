PWD := $(shell pwd)
BUILD_FLAG := CGO_ENABLED=0 GOOS=linux GOARCH=amd64

.PHONY: pb build run start stop

pb:
	(rm -r ./pbgo || true) && mkdir ./pbgo
	protoc -I . --go_out=plugins=grpc,:../microservice ./pb/*.proto

run: stop build start

build:
	mkdir output || true
	go mod tidy
	$(BUILD_FLAG) go build -o output/time-server time-service/main.go
	$(BUILD_FLAG) go build -o output/gateway-server gateway-service/main.go

start:
	ln -s $(PWD)/conf/all.yml $(PWD)/all.yml || true
	mkdir logs || true
	output/time-server > logs/time-server.log &
	output/gateway-server logs/gateway-server.log &

stop:
	(ps a | grep gateway-server | grep -v grep | awk '{print $1}' | xargs kill -s INT || true)
	(ps a | grep time-server | grep -v grep | awk '{print $1}' | xargs kill -s INT || true)

docker:
	docker build . -f build/gateway.dockerfile -t registry:5000/gateway-server
	docker push registry:5000/gateway-server
	docker build . -f build/time.dockerfile -t registry:5000/time-server
	docker push registry:5000/time-server

deploy:
	kubectl delete deploy time-app || true
	kubectl apply -f build/time.yml
	kubectl delete deploy gateway-app || true
	kubectl apply -f build/gateway.yml

