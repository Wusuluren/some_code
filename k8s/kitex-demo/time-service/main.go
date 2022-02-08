package main

import (
	timepb "kitex-demo/time-service/kitex_gen/pbgo/timepb/timeservice"
	"log"
)

func main() {
	svr := timepb.NewServer(new(TimeServiceImpl))

	err := svr.Run()

	if err != nil {
		log.Println(err.Error())
	}
}
