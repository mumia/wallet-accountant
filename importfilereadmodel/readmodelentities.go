package importfilereadmodel

import (
	"time"
	"walletaccountant/account"
	"walletaccountant/importfile"
)

type Entity struct {
	ImportFileId   *importfile.Id      `json:"importFileId" bson:"_id"`
	AccountId      *account.Id         `json:"accountId" bson:"account_id"`
	Filename       string              `json:"filename" bson:"filename"`
	FileType       importfile.FileType `json:"fileType" bson:"file_type"`
	ImportDate     time.Time           `json:"importDate" bson:"import_date"`
	StartParseDate *time.Time          `json:"startParseDate" bson:"start_parse_date,omitempty"`
	EndParseDate   *time.Time          `json:"endParseDate" bson:"end_parse_date,omitempty"`
	FailParseDate  *time.Time          `json:"failParseDate" bson:"fail_parse_date,omitempty"`
	State          importfile.State    `json:"state" bson:"state"`
	Code           *string             `json:"code" bson:"code,omitempty"`
	Reason         *string             `json:"reason" bson:"reason,omitempty"`
	RowCount       int                 `json:"rowCount" bson:"row_count"`
}

type FileRowsEntity struct {
	ImportFileId *importfile.Id  `json:"importFileId" bson:"_id"`
	AccountId    *account.Id     `json:"accountId" bson:"account_id"`
	Filename     string          `json:"filename" bson:"filename"`
	RowCount     int             `json:"rowCount" bson:"row_count"`
	Rows         []FileRowEntity `json:"rows" bson:"rows"`
}

type FileRowEntity struct {
	FileDataRowId *importfile.DataRowId   `json:"fileDataRowId" bson:"_id"`
	Date          time.Time               `json:"date" bson:"date"`
	Description   string                  `json:"description" bson:"description"`
	Amount        int64                   `json:"amount" bson:"amount"`
	RawData       map[string]any          `json:"rawData" bson:"raw_data"`
	State         importfile.DataRowState `json:"state" bson:"state"`
}
