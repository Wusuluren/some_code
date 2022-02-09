setup:
	go get -u github.com/TarsCloud/TarsGo/tars/tools/tars2go
	go mod init tarsgo-demo

generate:
    tars2go --outdir=./timepb time.tars

run:
    cd time-service
    go run main.go --config=config.conf &
    cd ..
    cd gateway-service
    go run main.go --config=config.conf &
    cd ..
    curl "127.0.0.1:10025/time/getTime?timezone=Local"