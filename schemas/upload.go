package schemas

type UploadQuery struct {
	Filename string `form:"fileName"`
	PartNo   int    `form:"partNo,omitempty"`
}

type UploadPartOut struct {
	Name   string `json:"name"`
	PartNo int    `json:"partNo"`
	Url    string `json:"url"`
	Size   int64  `json:"size"`
}

type UploadPart struct {
	Name   string `json:"name" binding:"required"`
	Url    string `json:"url" binding:"required"`
	PartNo int    `json:"partNo" binding:"required"`
	Size   int64  `json:"size" binding:"required"`
}
