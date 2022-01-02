package main

import (
	"net/http"

	"github.com/emicklei/go-restful/v3"
	"k8s.io/klog/v2"
)

func main() {
	var k Kubectl
	k.Init()

	ws := new(restful.WebService)
	ws.Path("/kubectl")
	ws.Route(ws.GET("/get/{resource}").To(k.Get))
	ws.Route(ws.GET("/get/{resource}/{name}").To(k.Get))

	handler := restful.NewContainer()
	handler.Add(ws)

	klog.Info("start...")
	http.ListenAndServe(":5211", handler)
}
