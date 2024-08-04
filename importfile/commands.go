package importfile

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/account"
	"walletaccountant/commands"
	"walletaccountant/eventstoredb"
	"walletaccountant/ledger"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

// Static type check that interface is implemented
var _ eventhorizon.Command = &RegisterNewImportFile{}
var _ eventhorizon.Command = &StartFileParse{}
var _ eventhorizon.Command = &RestartFileParse{}
var _ eventhorizon.Command = &EndFileParse{}
var _ eventhorizon.Command = &FailFileParse{}
var _ eventhorizon.Command = &AddFileDataRow{}
var _ eventhorizon.Command = &VerifyFileDataRow{}
var _ eventhorizon.Command = &InvalidateFileDataRow{}

const (
	RegisterNewImportFileCommand                           = eventhorizon.CommandType("register_new_import_file")
	StartFileParseCommand                                  = eventhorizon.CommandType("start_file_parse")
	RestartFileParseCommand                                = eventhorizon.CommandType("restart_file_parse")
	EndFileParseCommand                                    = eventhorizon.CommandType("end_file_parse")
	FailFileParseCommand                                   = eventhorizon.CommandType("fail_file_parse")
	AddFileDataRowCommand                                  = eventhorizon.CommandType("add_file_data_row")
	VerifyFileDataRowCommand                               = eventhorizon.CommandType("verify_file_data_row_command")
	InvalidateFileDataRowCommand                           = eventhorizon.CommandType("invalidate_file_data_row_command")
	RegisterAccountMovementIdForVerifiedFileDataRowCommand = eventhorizon.CommandType("register_account_movement_verified_file_data_row_command")
)

func RegisterCommandHandler(
	eventStoreFactory eventstoredb.EventStoreCreator,
	commandHandler eventhorizon.CommandHandler,
) error {
	return commands.RegisterCommandTypes(
		eventStoreFactory,
		commandHandler,
		AggregateType,
		[]commands.CommandAndType{
			{
				Command:     &RegisterNewImportFile{},
				CommandType: RegisterNewImportFileCommand,
			},
			{
				Command:     &StartFileParse{},
				CommandType: StartFileParseCommand,
			},
			{
				Command:     &RestartFileParse{},
				CommandType: RestartFileParseCommand,
			},
			{
				Command:     &EndFileParse{},
				CommandType: EndFileParseCommand,
			},
			{
				Command:     &FailFileParse{},
				CommandType: FailFileParseCommand,
			},
			{
				Command:     &AddFileDataRow{},
				CommandType: AddFileDataRowCommand,
			},
			{
				Command:     &VerifyFileDataRow{},
				CommandType: VerifyFileDataRowCommand,
			},
			{
				Command:     &InvalidateFileDataRow{},
				CommandType: InvalidateFileDataRowCommand,
			},
			{
				Command:     &RegisterAccountMovementIdForVerifiedFileDataRow{},
				CommandType: RegisterAccountMovementIdForVerifiedFileDataRowCommand,
			},
		},
	)
}

type RegisterNewImportFile struct {
	ImportFileId Id         `json:"import_file_id"`
	AccountId    account.Id `json:"account_id"`
	Filename     string     `json:"filename"`
	FileType     FileType   `json:"file_type"`
}

func (r RegisterNewImportFile) AggregateID() uuid.UUID {
	return uuid.UUID(r.ImportFileId)
}

func (r RegisterNewImportFile) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (r RegisterNewImportFile) CommandType() eventhorizon.CommandType {
	return RegisterNewImportFileCommand
}

type StartFileParse struct {
	ImportFileId Id `json:"import_file_id"`
}

func (s StartFileParse) AggregateID() uuid.UUID {
	return uuid.UUID(s.ImportFileId)
}

func (s StartFileParse) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s StartFileParse) CommandType() eventhorizon.CommandType {
	return StartFileParseCommand
}

type RestartFileParse struct {
	ImportFileId Id `json:"import_file_id"`
}

func (r RestartFileParse) AggregateID() uuid.UUID {
	return uuid.UUID(r.ImportFileId)
}

func (r RestartFileParse) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (r RestartFileParse) CommandType() eventhorizon.CommandType {
	return RestartFileParseCommand
}

type EndFileParse struct {
	ImportFileId Id `json:"import_file_id"`
}

func (s EndFileParse) AggregateID() uuid.UUID {
	return uuid.UUID(s.ImportFileId)
}

func (s EndFileParse) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s EndFileParse) CommandType() eventhorizon.CommandType {
	return EndFileParseCommand
}

type FailFileParse struct {
	ImportFileId Id     `json:"import_file_id"`
	Code         string `json:"code"`
	Reason       string `json:"reason"`
}

func (s FailFileParse) AggregateID() uuid.UUID {
	return uuid.UUID(s.ImportFileId)
}

func (s FailFileParse) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s FailFileParse) CommandType() eventhorizon.CommandType {
	return FailFileParseCommand
}

type AddFileDataRow struct {
	ImportFileId  Id             `json:"import_file_id"`
	FileDataRowId DataRowId      `json:"file_data_row_id"`
	Date          time.Time      `json:"date"`
	Description   string         `json:"description"`
	Amount        int64          `json:"amount"`
	RawData       map[string]any `json:"raw_data"`
}

func (s AddFileDataRow) AggregateID() uuid.UUID {
	return uuid.UUID(s.ImportFileId)
}

func (s AddFileDataRow) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s AddFileDataRow) CommandType() eventhorizon.CommandType {
	return AddFileDataRowCommand
}

type VerifyFileDataRow struct {
	ImportFileId    Id                   `json:"import_file_id"`
	FileDataRowId   DataRowId            `json:"file_data_row_id"`
	MovementTypeId  *movementtype.Id     `json:"movement_type_id" eh:"optional"`
	SourceAccountId *account.Id          `json:"source_account_id" eh:"optional"`
	Description     string               `json:"description"`
	TagIds          []*tagcategory.TagId `json:"tag_ids"`
}

func (s VerifyFileDataRow) AggregateID() uuid.UUID {
	return uuid.UUID(s.ImportFileId)
}

func (s VerifyFileDataRow) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s VerifyFileDataRow) CommandType() eventhorizon.CommandType {
	return VerifyFileDataRowCommand
}

type InvalidateFileDataRow struct {
	ImportFileId  Id        `json:"import_file_id"`
	FileDataRowId DataRowId `json:"file_data_row_id"`
	Reason        string    `json:"reason"`
}

func (s InvalidateFileDataRow) AggregateID() uuid.UUID {
	return uuid.UUID(s.ImportFileId)
}

func (s InvalidateFileDataRow) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s InvalidateFileDataRow) CommandType() eventhorizon.CommandType {
	return InvalidateFileDataRowCommand
}

type RegisterAccountMovementIdForVerifiedFileDataRow struct {
	ImportFileId      Id                       `json:"import_file_id"`
	FileDataRowId     DataRowId                `json:"file_data_row_id"`
	AccountMovementId ledger.AccountMovementId `json:"account_movement_id"`
}

func (s RegisterAccountMovementIdForVerifiedFileDataRow) AggregateID() uuid.UUID {
	return uuid.UUID(s.ImportFileId)
}

func (s RegisterAccountMovementIdForVerifiedFileDataRow) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s RegisterAccountMovementIdForVerifiedFileDataRow) CommandType() eventhorizon.CommandType {
	return RegisterAccountMovementIdForVerifiedFileDataRowCommand
}
