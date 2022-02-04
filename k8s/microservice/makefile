PWD := $(shell pwd)
BUILD_FLAG := CGO_ENABLED=0 GOOS=linux GOARCH=amd64

.PHONY: pb build run start stop all

pb:
	(rm -r ./pbgo || true) && mkdir ./pbgo
	protoc -I . --go_out=plugins=grpc,:../microservice ./pb/*.proto

run: stop build start

build: pb
	ln -s $(PWD)/conf/all.yml $(PWD)/all.yml || true
	mkdir output || true
	go mod tidy
	$(BUILD_FLAG) go build -o output/time-server time-service/main.go
	$(BUILD_FLAG) go build -o output/gateway-server gateway-service/main.go

start:
	mkdir logs || true
	output/time-server > logs/time-server.log &
	output/gateway-server logs/gateway-server.log &

stop:
	(ps a | grep gateway-server | grep -v grep | awk '{print $1}' | xargs kill -s INT || true)
	(ps a | grep time-server | grep -v grep | awk '{print $1}' | xargs kill -s INT || true)

docker:
	docker build . -f artifacts/gateway.dockerfile -t registry:5000/gateway-server
	docker push registry:5000/gateway-server
	docker build . -f artifacts/time.dockerfile -t registry:5000/time-server
	docker push registry:5000/time-server

deploy:
	kubectl delete deploy time-app || true
	kubectl delete service time-service || true
	kubectl apply -f artifacts/time.yml

	kubectl delete deploy gateway-app || true
	kubectl delete service gateway-service || true
	kubectl apply -f artifacts/gateway.yml

	kubectl delete ingress gateway-ingress || true
	kubectl apply -f artifacts/gateway-ingress.yml

all: build docker deploy