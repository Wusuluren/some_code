package time

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"gozero-demo/gateway-service/internal/logic/time"
	"gozero-demo/gateway-service/internal/svc"
	"gozero-demo/gateway-service/internal/types"
)

func GetTimeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetTimeReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Error(w, err)
			return
		}

		l := time.NewGetTimeLogic(r.Context(), svcCtx)
		resp, err := l.GetTime(req)
		if err != nil {
			httpx.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
