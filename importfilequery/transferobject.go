package importfilequery

type importFileRequest struct {
	ImportFileId string `uri:"importFileId"  binding:"required,uuid"`
}
