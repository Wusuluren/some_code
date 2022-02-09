package main

import (
	"fmt"
	"github.com/TarsCloud/TarsGo/tars"
	"net/http"
	"tarsgo-demo/timepb/Time"
)

func main() {
	mux := &tars.TarsHttpMux{}
	mux.HandleFunc("/time/getTime", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		timezone := r.FormValue("timezone")

		comm := tars.NewCommunicator()
		obj := "Time.Time.TimeOb@tcp -h 127.0.0.1 -p 10015 -t 60000"
		app := new(Time.Time)
		comm.StringToProxy(obj, app)
		var req string = timezone
		var res string
		ret, err := app.GetTime(req, &res)
		if err != nil {
			fmt.Println(err)
			w.Write([]byte(err.Error()))
			w.WriteHeader(500)
			return
		}
		fmt.Println(ret, res)
		w.Write([]byte(res))
		w.WriteHeader(200)
	})

	cfg := tars.GetServerConfig()
	tars.AddHttpServant(mux, cfg.App+"."+cfg.Server+".TimeObj") //Register http server
	tars.Run()
}
