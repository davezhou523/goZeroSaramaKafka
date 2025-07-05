package kafka

type ChunkPayload struct {
	FileId      string `json:"file_id"`      //文件id
	Index       int    `json:"index"`        //文件拆分索引号
	TotalChunks int    `json:"total_chunks"` //总索引号
	Data        []byte `json:"data"`         //拆分数据
	Category    string `json:"category"`     //数据分类：图片、文件、文本
}
