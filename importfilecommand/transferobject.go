package importfilecommand

import (
	"mime/multipart"
)

type RegisterNewImportFileFormTransferObject struct {
	AccountId string                `form:"accountId" binding:"required,uuid"`
	File      *multipart.FileHeader `form:"filename" binding:"required"`
}

type RegisterNewImportFileTransferObject struct {
	AccountId string
	Filename  string
}

type VerifyFileDataRowTransferObject struct {
	ImportFileId    string   `json:"importFileId" binding:"required"`
	FileDataRowId   string   `json:"fileDataRowId" binding:"required"`
	MovementTypeId  *string  `json:"movementTypeId" binding:"omitempty,uuid"`
	SourceAccountId *string  `json:"sourceAccountId" binding:"omitempty,uuid"`
	Description     string   `json:"description" binding:"required"`
	TagIds          []string `json:"tagIds" binding:"required"`
}

type InvalidateFileDataRowTransferObject struct {
	ImportFileId  string `json:"importFileId" binding:"required"`
	FileDataRowId string `json:"fileDataRowId" binding:"required"`
	Reason        string `json:"reason" binding:"required"`
}
