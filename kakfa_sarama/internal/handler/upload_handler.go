package handler

import (
	"github.com/zeromicro/go-zero/core/errorx"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"gozero1/kakfa_sarama/internal/logic"
	"gozero1/kakfa_sarama/internal/svc"
)

func UploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 解析表单（包含文件）
		err := r.ParseMultipartForm(10 << 20) // 10MB 内存缓冲
		if err != nil {
			httpx.Error(w, errorx.Wrap(err, "解析表单失败"))
			return
		}
		// 获取文件流
		file, header, err := r.FormFile("file") // "file" 是前端表单字段名
		if err != nil {
			httpx.Error(w, errorx.Wrap(err, "获取文件失败"+header.Filename))
			return
		}
		defer file.Close()
		// 调用 Logic 层处理业务
		l := logic.NewUploadLogic(r.Context(), svcCtx)
		resp, err := l.SendFileToKafka(file, header.Filename)
		if err != nil {
			httpx.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}

	}
}
