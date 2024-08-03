package importfilecommand_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/eventstoredb"
	"walletaccountant/importfile"
	"walletaccountant/importfile/bankfileparser"
	"walletaccountant/importfilecommand"
	"walletaccountant/importfilereadmodel"
	"walletaccountant/mocks"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

func setupCommandMediatorTest() {
	commands := []func() eventhorizon.Command{
		func() eventhorizon.Command { return &importfile.RegisterNewImportFile{} },
		func() eventhorizon.Command { return &importfile.StartFileParse{} },
		func() eventhorizon.Command { return &importfile.RestartFileParse{} },
		func() eventhorizon.Command { return &importfile.EndFileParse{} },
		func() eventhorizon.Command { return &importfile.FailFileParse{} },
		func() eventhorizon.Command { return &importfile.AddFileDataRow{} },
		func() eventhorizon.Command { return &importfile.VerifyFileDataRow{} },
		func() eventhorizon.Command { return &importfile.InvalidateFileDataRow{} },
		func() eventhorizon.Command { return &importfile.RegisterAccountMovementIdForVerifiedFileDataRow{} },
	}

	for _, command := range commands {
		eventhorizon.RegisterCommand(command)
	}
}

func tearDownCommandMediatorTest() {
	eventhorizon.UnregisterCommand(importfile.RegisterNewImportFileCommand)
	eventhorizon.UnregisterCommand(importfile.StartFileParseCommand)
	eventhorizon.UnregisterCommand(importfile.RestartFileParseCommand)
	eventhorizon.UnregisterCommand(importfile.EndFileParseCommand)
	eventhorizon.UnregisterCommand(importfile.FailFileParseCommand)
	eventhorizon.UnregisterCommand(importfile.AddFileDataRowCommand)
	eventhorizon.UnregisterCommand(importfile.VerifyFileDataRowCommand)
	eventhorizon.UnregisterCommand(importfile.InvalidateFileDataRowCommand)
	eventhorizon.UnregisterCommand(importfile.RegisterAccountMovementIdForVerifiedFileDataRowCommand)
}

func TestCommandMediator_RegisterNewImportFileCommand(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	err := os.Setenv("FILE_UPLOAD_PATH", fileUploadPath)
	requires.NoError(err)

	transferObject := importfilecommand.RegisterNewImportFileTransferObject{
		AccountId: accountUUID1.String(),
		Filename:  "test.csv",
	}

	expectedCommand := &importfile.RegisterNewImportFile{
		ImportFileId: *importfile.IdFromUUID(importFileUUID1),
		AccountId:    *account.IdFromUUID(accountUUID1),
		Filename:     "test.csv",
		FileType:     importfile.CSV,
	}

	idCreator := &eventstoredb.IdCreatorMock{
		NewFn: func() uuid.UUID {
			return importFileUUID1
		},
	}

	var parsers []bankfileparser.BankFileParser
	parsers = append(parsers, bankfileparser.NewBcpCSVParser())
	parsers = append(parsers, bankfileparser.NewN26CSVParser())
	parsers = append(parsers, bankfileparser.NewDeutscheBankCSVParser())

	expectedFullUploadPath := fmt.Sprintf("%s/test.csv", fileUploadPath)

	var accountByIdCalled int
	var fileOpenCalled int
	var fileSaveCalled int
	successTestCases := [...]struct {
		testName                   string
		accountReadModelRepository accountreadmodel.ReadModeler
		fileHandler                common.Filer
	}{
		{
			testName:                   "successfully handles BCP csv file upload",
			accountReadModelRepository: accountReadModelRepositoryMock(t, &accountByIdCalled, account.BCP),
			fileHandler: filerMock(
				t,
				&fileOpenCalled,
				&fileSaveCalled,
				expectedFullUploadPath,
				validBCPCSVReader,
				expectedSemiColonCSVContent,
			),
		},
		{
			testName:                   "successfully handles N26 csv file upload",
			accountReadModelRepository: accountReadModelRepositoryMock(t, &accountByIdCalled, account.N26),
			fileHandler: filerMock(
				t,
				&fileOpenCalled,
				&fileSaveCalled,
				expectedFullUploadPath,
				validN26CSVReader,
				expectedColonCSVContent,
			),
		},
		{
			testName:                   "successfully handles DB csv file upload",
			accountReadModelRepository: accountReadModelRepositoryMock(t, &accountByIdCalled, account.DB),
			fileHandler: filerMock(
				t,
				&fileOpenCalled,
				&fileSaveCalled,
				expectedFullUploadPath,
				validDBCSVReader,
				expectedSemiColonCSVContent,
			),
		},
	}
	for _, testCase := range successTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			accountByIdCalled = 0
			commandHandlerCalled := 0
			fileOpenCalled = 0
			fileSaveCalled = 0

			commandHandler := &mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					commandHandlerCalled++

					asserts.Equal(expectedCommand, command)

					return nil
				},
			}

			commandMediator := importfilecommand.NewCommandMediator(
				parsers,
				commandHandler,
				&importfilereadmodel.ReadModelRepositoryMock{},
				testCase.accountReadModelRepository,
				idCreator,
				testCase.fileHandler,
				zaptest.NewLogger(t),
			)

			actualId, err := commandMediator.RegisterNewImportFile(&gin.Context{}, transferObject)
			requires.Nil(err)

			asserts.Equal(importFileUUID1.String(), actualId.String())
			asserts.Equal(1, accountByIdCalled)
			asserts.Equal(1, commandHandlerCalled)
			asserts.Equal(1, fileOpenCalled)
			asserts.Equal(1, fileSaveCalled)
		})
	}
}

func TestCommandMediator_StartFileParseCommand(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	expectedCommand := &importfile.StartFileParse{
		ImportFileId: *importfile.IdFromUUID(importFileUUID1),
	}

	t.Run("successfully starts parsing an imported file", func(t *testing.T) {
		getByIdFnCalled := 0
		commandHandlerCalled := 0

		importFileRepository := &importfilereadmodel.ReadModelRepositoryMock{
			GetByIdFn: func(ctx context.Context, importFileId *importfile.Id) (*importfilereadmodel.Entity, error) {
				getByIdFnCalled++

				return &importfilereadmodel.Entity{
					ImportFileId:   importfile.IdFromUUID(importFileUUID1),
					AccountId:      account.IdFromUUID(accountUUID1),
					Filename:       "",
					FileType:       "",
					ImportDate:     time.Now(),
					StartParseDate: nil,
					EndParseDate:   nil,
					FailParseDate:  nil,
					State:          "",
					Code:           nil,
					Reason:         nil,
					RowCount:       0,
				}, nil
			},
		}

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				commandHandlerCalled++

				asserts.Equal(expectedCommand, command)

				return nil
			},
		}

		commandMediator := importfilecommand.NewCommandMediator(
			[]bankfileparser.BankFileParser{},
			commandHandler,
			importFileRepository,
			&accountreadmodel.ReadModelRepositoryMock{},
			&eventstoredb.IdCreatorMock{},
			&common.FileHandlerMock{},
			zaptest.NewLogger(t),
		)

		err := commandMediator.StartFileParse(&gin.Context{}, importfile.IdFromUUID(importFileUUID1))
		requires.Nil(err)

		asserts.Equal(1, getByIdFnCalled)
		asserts.Equal(1, commandHandlerCalled)
	})
}

func TestCommandMediator_RestartFileParseCommand(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	expectedCommand := &importfile.RestartFileParse{
		ImportFileId: *importfile.IdFromUUID(importFileUUID1),
	}

	t.Run("successfully restarts parsing an imported file", func(t *testing.T) {
		getByIdFnCalled := 0
		commandHandlerCalled := 0

		importFileRepository := &importfilereadmodel.ReadModelRepositoryMock{
			GetByIdFn: func(ctx context.Context, importFileId *importfile.Id) (*importfilereadmodel.Entity, error) {
				getByIdFnCalled++

				return &importfilereadmodel.Entity{
					ImportFileId:   importfile.IdFromUUID(importFileUUID1),
					AccountId:      account.IdFromUUID(accountUUID1),
					Filename:       "",
					FileType:       "",
					ImportDate:     time.Now(),
					StartParseDate: nil,
					EndParseDate:   nil,
					FailParseDate:  nil,
					State:          importfile.ParsingFailed,
					Code:           nil,
					Reason:         nil,
					RowCount:       0,
				}, nil
			},
		}

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				commandHandlerCalled++

				asserts.Equal(expectedCommand, command)

				return nil
			},
		}

		commandMediator := importfilecommand.NewCommandMediator(
			[]bankfileparser.BankFileParser{},
			commandHandler,
			importFileRepository,
			&accountreadmodel.ReadModelRepositoryMock{},
			&eventstoredb.IdCreatorMock{},
			&common.FileHandlerMock{},
			zaptest.NewLogger(t),
		)

		err := commandMediator.RestartFileParse(&gin.Context{}, importfile.IdFromUUID(importFileUUID1))
		requires.Nil(err)

		asserts.Equal(1, getByIdFnCalled)
		asserts.Equal(1, commandHandlerCalled)
	})
}

func TestCommandMediator_EndFileParseCommand(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	expectedCommand := &importfile.EndFileParse{
		ImportFileId: *importfile.IdFromUUID(importFileUUID1),
	}

	t.Run("successfully ends parsing an imported file", func(t *testing.T) {
		getByIdFnCalled := 0
		commandHandlerCalled := 0

		importFileRepository := &importfilereadmodel.ReadModelRepositoryMock{
			GetByIdFn: func(ctx context.Context, importFileId *importfile.Id) (*importfilereadmodel.Entity, error) {
				getByIdFnCalled++

				return &importfilereadmodel.Entity{
					ImportFileId:   importfile.IdFromUUID(importFileUUID1),
					AccountId:      account.IdFromUUID(accountUUID1),
					Filename:       "",
					FileType:       "",
					ImportDate:     time.Now(),
					StartParseDate: nil,
					EndParseDate:   nil,
					FailParseDate:  nil,
					State:          importfile.ParsingStarted,
					Code:           nil,
					Reason:         nil,
					RowCount:       0,
				}, nil
			},
		}

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				commandHandlerCalled++

				asserts.Equal(expectedCommand, command)

				return nil
			},
		}

		commandMediator := importfilecommand.NewCommandMediator(
			[]bankfileparser.BankFileParser{},
			commandHandler,
			importFileRepository,
			&accountreadmodel.ReadModelRepositoryMock{},
			&eventstoredb.IdCreatorMock{},
			&common.FileHandlerMock{},
			zaptest.NewLogger(t),
		)

		err := commandMediator.EndFileParse(&gin.Context{}, importfile.IdFromUUID(importFileUUID1))
		requires.Nil(err)

		asserts.Equal(1, getByIdFnCalled)
		asserts.Equal(1, commandHandlerCalled)
	})
}

func TestCommandMediator_FailFileParseCommand(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	expectedCommand := &importfile.FailFileParse{
		ImportFileId: *importfile.IdFromUUID(importFileUUID1),
		Code:         "a_code",
		Reason:       "a reason",
	}

	t.Run("successfully fails parsing an imported file", func(t *testing.T) {
		getByIdFnCalled := 0
		commandHandlerCalled := 0

		importFileRepository := &importfilereadmodel.ReadModelRepositoryMock{
			GetByIdFn: func(ctx context.Context, importFileId *importfile.Id) (*importfilereadmodel.Entity, error) {
				getByIdFnCalled++

				return &importfilereadmodel.Entity{
					ImportFileId:   importfile.IdFromUUID(importFileUUID1),
					AccountId:      account.IdFromUUID(accountUUID1),
					Filename:       "",
					FileType:       "",
					ImportDate:     time.Now(),
					StartParseDate: nil,
					EndParseDate:   nil,
					FailParseDate:  nil,
					State:          importfile.ParsingStarted,
					Code:           nil,
					Reason:         nil,
					RowCount:       0,
				}, nil
			},
		}

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				commandHandlerCalled++

				asserts.Equal(expectedCommand, command)

				return nil
			},
		}

		commandMediator := importfilecommand.NewCommandMediator(
			[]bankfileparser.BankFileParser{},
			commandHandler,
			importFileRepository,
			&accountreadmodel.ReadModelRepositoryMock{},
			&eventstoredb.IdCreatorMock{},
			&common.FileHandlerMock{},
			zaptest.NewLogger(t),
		)

		err := commandMediator.FailFileParse(
			&gin.Context{},
			importfile.IdFromUUID(importFileUUID1),
			"a_code",
			"a reason",
		)
		requires.Nil(err)

		asserts.Equal(1, getByIdFnCalled)
		asserts.Equal(1, commandHandlerCalled)
	})
}

func TestCommandMediator_AddFileDataRowCommand(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	date := time.Now()

	expectedCommand := &importfile.AddFileDataRow{
		ImportFileId:  *importfile.IdFromUUID(importFileUUID1),
		FileDataRowId: *importfile.DataRowIdFromUUID(fileDataRowUUID1),
		Date:          date,
		Description:   "a description",
		Amount:        100,
		RawData: map[string]any{
			"something": 10,
			"else":      "value",
		},
	}

	t.Run("successfully adds file data row", func(t *testing.T) {
		commandHandlerCalled := 0

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				commandHandlerCalled++

				asserts.Equal(expectedCommand, command)

				return nil
			},
		}

		commandMediator := importfilecommand.NewCommandMediator(
			[]bankfileparser.BankFileParser{},
			commandHandler,
			&importfilereadmodel.ReadModelRepositoryMock{},
			&accountreadmodel.ReadModelRepositoryMock{},
			&eventstoredb.IdCreatorMock{},
			&common.FileHandlerMock{},
			zaptest.NewLogger(t),
		)

		err := commandMediator.AddFileDataRow(
			&gin.Context{},
			importfile.IdFromUUID(importFileUUID1),
			&bankfileparser.BankFileDataRow{
				ImportFileId: importfile.IdFromUUID(importFileUUID1),
				AccountId:    account.IdFromUUID(accountUUID1),
				DataRowId:    importfile.DataRowIdFromUUID(fileDataRowUUID1),
				Date:         date,
				Description:  "a description",
				Amount:       100,
				RawData: map[string]any{
					"something": 10,
					"else":      "value",
				},
			},
		)
		requires.Nil(err)

		asserts.Equal(1, commandHandlerCalled)
	})
}

func TestCommandMediator_VerifyFileDataRowCommand(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	successTestCases := [...]struct {
		testName        string
		movementTypeId  *string
		sourceAccountId *string
	}{
		{
			testName:        "successfully adds file data row, no movement type or source account",
			movementTypeId:  nil,
			sourceAccountId: nil,
		},
		{
			testName:        "successfully adds file data row, with movement type, no source account",
			movementTypeId:  &movementTypeUUiDString1,
			sourceAccountId: nil,
		},
		{
			testName:        "successfully adds file data row, no movement type, with source account",
			movementTypeId:  nil,
			sourceAccountId: &sourceAccountUUiDString1,
		},
		{
			testName:        "successfully adds file data row, with both movement type and source account",
			movementTypeId:  &movementTypeUUiDString1,
			sourceAccountId: &sourceAccountUUiDString1,
		},
	}
	for _, testCase := range successTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			commandHandlerCalled := 0

			var movementTypeId *movementtype.Id
			if testCase.movementTypeId != nil {
				movementTypeId = movementtype.IdFromUUIDString(*testCase.movementTypeId)
			}

			var sourceAccountId *account.Id
			if testCase.sourceAccountId != nil {
				sourceAccountId = account.IdFromUUIDString(*testCase.sourceAccountId)
			}

			expectedCommand := &importfile.VerifyFileDataRow{
				ImportFileId:    *importfile.IdFromUUID(importFileUUID1),
				FileDataRowId:   *importfile.DataRowIdFromUUID(fileDataRowUUID1),
				MovementTypeId:  movementTypeId,
				SourceAccountId: sourceAccountId,
				Description:     "some description",
				TagIds: []*tagcategory.TagId{
					tagcategory.TagIdFromUUID(tagUUID1),
				},
			}

			commandHandler := &mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					commandHandlerCalled++

					asserts.Equal(expectedCommand, command)

					return nil
				},
			}

			commandMediator := importfilecommand.NewCommandMediator(
				[]bankfileparser.BankFileParser{},
				commandHandler,
				&importfilereadmodel.ReadModelRepositoryMock{},
				&accountreadmodel.ReadModelRepositoryMock{},
				&eventstoredb.IdCreatorMock{},
				&common.FileHandlerMock{},
				zaptest.NewLogger(t),
			)

			err := commandMediator.VerifyFileDataRow(
				&gin.Context{},
				importfilecommand.VerifyFileDataRowTransferObject{
					ImportFileId:    importFileUUID1.String(),
					FileDataRowId:   fileDataRowUUID1.String(),
					MovementTypeId:  testCase.movementTypeId,
					SourceAccountId: testCase.sourceAccountId,
					Description:     "some description",
					TagIds: []string{
						tagUUID1.String(),
					},
				},
			)
			requires.Nil(err)

			asserts.Equal(1, commandHandlerCalled)
		})
	}
}

func accountReadModelRepositoryMock(
	t *testing.T,
	accountByIdCalled *int,
	bankName account.BankName,
) accountreadmodel.ReadModeler {
	return &accountreadmodel.ReadModelRepositoryMock{
		GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
			*accountByIdCalled++

			require.Equal(t, 1, *accountByIdCalled)

			assert.Equal(t, accountUUID1.String(), accountId.String())

			return &accountreadmodel.Entity{
				AccountId:           account.IdFromUUID(accountUUID1),
				BankName:            bankName,
				BankNameExtra:       nil,
				Name:                "an account",
				AccountType:         common.Checking,
				StartingBalance:     0,
				StartingBalanceDate: time.Now(),
				Currency:            account.EUR,
				Notes:               nil,
				ActiveMonth: accountreadmodel.EntityActiveMonth{
					Month: time.Month(12),
					Year:  uint(2024),
				},
			}, nil
		},
	}
}

func filerMock(
	t *testing.T,
	fileOpenCalled *int,
	fileSaveCalled *int,
	expectedFullUploadPath string,
	validCSVReader func() *testFile,
	expectedCSVContent func() string,
) common.Filer {
	return &common.FileHandlerMock{
		OpenFn: func(filePath string) (io.ReadCloser, error) {
			*fileOpenCalled++

			assert.Equal(t, expectedFullUploadPath, filePath)

			return validCSVReader(), nil
		},
		SaveFn: func(filePath string, content *bytes.Buffer) error {
			*fileSaveCalled++

			assert.Equal(t, expectedFullUploadPath, filePath)
			assert.Equal(t, expectedCSVContent(), content.String())

			return nil
		},
	}
}

func validBCPCSVReader() *testFile {
	content := `To remove 1
To remove 2
To remove 3
To remove 4
To remove 5
To remove 6
To remove 7
To remove 8
Valid;Fields
with;values
To remove 1
To remove 2
To remove 3`

	return newTestFile(strings.NewReader(content))
}

func validN26CSVReader() *testFile {
	content := `Valid,Fields
with,values`

	return newTestFile(strings.NewReader(content))
}

func validDBCSVReader() *testFile {
	content := `To remove 1
To remove 2
To remove 3
To remove 4
Valid;Fields
with;values
To remove 1
To remove 2`

	return newTestFile(strings.NewReader(content))
}

func expectedSemiColonCSVContent() string {
	return `Valid;Fields
with;values`
}

func expectedColonCSVContent() string {
	return `Valid,Fields
with,values`
}
