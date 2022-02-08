setup:
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest

generate-gateway:
	kratos new gateway-service
	cd gateway-service
	make all
	kratos proto add api/time/time.proto
	kratos proto client api/time/time.proto
	kratos proto server api/time/time.proto -t internal/service

generate-time:
	kratos new time-service
	cd time-service
	make all
	kratos proto add api/time/time.proto
	kratos proto client api/time/time.proto
	kratos proto server api/time/time.proto -t internal/service

run:
    cd time-service; kratos run &; cd ..
    cd gateway-service; kratos run &; cd ..
    sleep 10
    curl  "127.0.0.1:8000/time/getTime?timezone=Local"