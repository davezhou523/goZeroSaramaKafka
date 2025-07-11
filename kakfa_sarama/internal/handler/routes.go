// Code generated by goctl. DO NOT EDIT.
// goctl 1.8.3

package handler

import (
	"net/http"

	"gozero1/kakfa_sarama/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/upload",
				Handler: UploadHandler(serverCtx),
			},
		},
	)
}
