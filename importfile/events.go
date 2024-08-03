package importfile

import (
	"github.com/looplab/eventhorizon"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

// Static type check that interface is implemented
var _ definitions.EventDataRegisters = &EventRegister{}

/*
NewImportFileRegistered

	-> FileParseStarted
		-> FileDataRowAdded ...
		-> FileParseEnded
		-> FileParseFailed
	-> FileParseFailed

FileParseFailed
	-> FileParseRestarted

FileDataRowAdded

	-> FileDataRowMarkedAsVerified (description, tags, movement type, source account)
		-> AccountMovementIdForVerifiedFileDataRowRegistered
	-> FileDataRowMarkedAsInvalid (reason)
*/

type FileType string

const (
	CSV FileType = "csv"
)

const (
	NewImportFileRegistered                           = eventhorizon.EventType("new_import_file_registered")
	FileParseStarted                                  = eventhorizon.EventType("file_parse_started")
	FileParseRestarted                                = eventhorizon.EventType("file_parse_restarted")
	FileParseEnded                                    = eventhorizon.EventType("file_parse_ended")
	FileParseFailed                                   = eventhorizon.EventType("file_parse_failed")
	FileDataRowAdded                                  = eventhorizon.EventType("file_data_row_added")
	FileDataRowMarkedAsVerified                       = eventhorizon.EventType("file_data_row_marked_as_verified")
	FileDataRowMarkedAsInvalid                        = eventhorizon.EventType("file_data_row_marked_as_invalid")
	AccountMovementIdForVerifiedFileDataRowRegistered = eventhorizon.EventType("account_movement_id_for_verified_file_data_row_registered")
)

type EventRegister struct {
}

func NewEventRegister() *EventRegister {
	return &EventRegister{}
}

func (eventList *EventRegister) Registers() []definitions.EventDataRegister {
	return []definitions.EventDataRegister{
		{
			EventType: NewImportFileRegistered,
			EventData: func() eventhorizon.EventData { return &NewImportFileRegisteredData{} },
		},
		{
			EventType: FileParseStarted,
			EventData: func() eventhorizon.EventData { return &FileParseStartedData{} },
		},
		{
			EventType: FileParseRestarted,
			EventData: func() eventhorizon.EventData { return &FileParseRestartedData{} },
		},
		{
			EventType: FileParseEnded,
			EventData: func() eventhorizon.EventData { return &FileParseEndedData{} },
		},
		{
			EventType: FileParseFailed,
			EventData: func() eventhorizon.EventData { return &FileParseFailedData{} },
		},
		{
			EventType: FileDataRowAdded,
			EventData: func() eventhorizon.EventData { return &FileDataRowAddedData{} },
		},
		{
			EventType: FileDataRowMarkedAsVerified,
			EventData: func() eventhorizon.EventData { return &FileDataRowMarkedAsVerifiedData{} },
		},
		{
			EventType: FileDataRowMarkedAsInvalid,
			EventData: func() eventhorizon.EventData { return &FileDataRowMarkedAsInvalidData{} },
		},
		{
			EventType: AccountMovementIdForVerifiedFileDataRowRegistered,
			EventData: func() eventhorizon.EventData { return &AccountMovementIdForVerifiedFileDataRowRegisteredData{} },
		},
	}
}

type NewImportFileRegisteredData struct {
	ImportFileId *Id         `json:"import_file_id"`
	AccountId    *account.Id `json:"account_id"`
	Filename     string      `json:"filename"`
	FileType     FileType    `json:"file_type"`
}

type FileParseStartedData struct {
	ImportFileId *Id `json:"import_file_id"`
}

type FileParseRestartedData struct {
	ImportFileId *Id `json:"import_file_id"`
}

type FileParseEndedData struct {
	ImportFileId      *Id   `json:"import_file_id"`
	TotalRowsImported uint8 `json:"total_rows_imported"`
}

type FileParseFailedData struct {
	ImportFileId *Id    `json:"import_file_id"`
	Code         string `json:"code"`
	Reason       string `json:"reason"`
}

type FileDataRowAddedData struct {
	ImportFileId  *Id            `json:"import_file_id"`
	FileDataRowId *DataRowId     `json:"file_data_row_id"`
	Date          time.Time      `json:"date"`
	Description   string         `json:"description"`
	Amount        int64          `json:"amount"`
	RawData       map[string]any `json:"raw_data"`
}

type FileDataRowMarkedAsVerifiedData struct {
	ImportFileId    *Id                  `json:"import_file_id"`
	FileDataRowId   *DataRowId           `json:"file_data_row_id"`
	Description     string               `json:"description"`
	MovementTypeId  *movementtype.Id     `json:"movement_type_id,omitempty"`
	SourceAccountId *account.Id          `json:"source_account_id,omitempty"`
	TagIds          []*tagcategory.TagId `json:"tag_ids"`
}

type FileDataRowMarkedAsInvalidData struct {
	ImportFileId  *Id        `json:"import_file_id"`
	FileDataRowId *DataRowId `json:"file_data_row_id"`
	Reason        string     `json:"reason"`
}

type AccountMovementIdForVerifiedFileDataRowRegisteredData struct {
	ImportFileId      *Id                             `json:"import_file_id"`
	FileDataRowId     *DataRowId                      `json:"file_data_row_id"`
	AccountMovementId *accountmonth.AccountMovementId `json:"account_movement_id"`
}
