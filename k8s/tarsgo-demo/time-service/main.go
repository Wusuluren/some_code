package main

import (
	"github.com/TarsCloud/TarsGo/tars"
	"tarsgo-demo/timepb/Time"
	"time"
)

type TimeImp struct {
}

func (imp *TimeImp) GetTime(sReq string, sResp *string) (ret int32, err error) {
	timezone := "Local"
	if sReq != "" {
		timezone = sReq
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return 1, err
	}
	now := time.Now().In(loc)
	*sResp = now.String()

	return 0, nil
}


func main() { //Init servant
	imp := new(TimeImp)                                    //New Imp
	app := new(Time.Time)                               //New init the A Tars
	cfg := tars.GetServerConfig()                           //Get Config File Object
	app.AddServant(imp, cfg.App+"."+cfg.Server+".TimeObj") //Register Servant
	tars.Run()
}

