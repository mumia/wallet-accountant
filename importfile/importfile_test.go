package importfile

import (
	"context"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/clock"
	"walletaccountant/ledger"
	"walletaccountant/tagcategory"
)

type stateFailureTestCase struct {
	testName string
	state    State
}

func setupImportFileTest(instants []clock.Instant) func(id uuid.UUID) eventhorizon.Aggregate {
	factory := NewFactory()
	factory.clock = clock.Freeze(instants, nil)

	return factory.Factory()
}

func setupAggregate(instants []clock.Instant, aggregateVersion int) *ImportFile {
	newAggregateFunc := setupImportFileTest(instants)

	importFileAggregate := newAggregateFunc(*setupImportFileId()).(*ImportFile)
	importFileAggregate.SetAggregateVersion(aggregateVersion)

	return importFileAggregate
}

func setupImportFileId() *Id {
	return IdFromUUIDString("6c686c88-3f90-494f-bbb4-9c412d514302")
}

func setupAccountId() *account.Id {
	return account.IdFromUUIDString("bbbcfa83-d879-4c24-b77d-a44e8ee572b2")
}

func setupAccountMovementId() *account.Id {
	return ledger.AccountMovementIdFromUUIDString("7d17c824-2b2f-41dd-a721-c564f29bd692")
}

func setupDataRowId(rawData map[string]any) (*DataRowId, error) {
	rowHash, err := hashstructure.Hash(rawData, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}

	return DataRowIdGenerate(setupImportFileId(), rowHash)
}

func setupTagId() *tagcategory.TagId {
	return tagcategory.TagIdFromUUIDString("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
}

func setupRawData() map[string]any {
	return map[string]any{
		"key":  "value",
		"key1": "value1",
		"key2": "value2",
	}
}

func TestImportFile_HandleCommand_RegisterNewImportFile(t *testing.T) {
	t.Parallel()

	asserts := assert.New(t)
	requires := require.New(t)

	instants := []clock.Instant{
		{"register new import file", time.Now()},
	}
	newAggregateFunc := setupImportFileTest(instants)

	importFileAggregate := newAggregateFunc(*setupImportFileId()).(*ImportFile)
	importFileAggregate.SetAggregateVersion(0)

	command := createRegisterNewImportFileCommand()
	expectedEvent := createNewImportFileRegisteredEvent(instants[0].Instant)

	t.Run("successfully register new import file", func(t *testing.T) {
		err := importFileAggregate.HandleCommand(context.Background(), command)
		requires.NoError(err)

		uncommittedEvents := importFileAggregate.UncommittedEvents()
		asserts.Equal(1, len(uncommittedEvents))
		asserts.Equal(expectedEvent, uncommittedEvents[0])
		asserts.Equal(expectedEvent.EventType(), uncommittedEvents[0].EventType())
		asserts.Equal(expectedEvent.AggregateType(), uncommittedEvents[0].AggregateType())
		asserts.Equal(expectedEvent.Data(), uncommittedEvents[0].Data())
	})

	t.Run("fails to register new import file, because import file already registered", func(t *testing.T) {
		importFileAggregate.SetAggregateVersion(1)

		err := importFileAggregate.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})
}

func TestImportFile_HandleCommand_StartFileParse(t *testing.T) {
	t.Parallel()

	messagePrefix := "start file parse"
	instants := []clock.Instant{
		{messagePrefix, time.Now()},
	}

	command := createStartFileParseCommand()
	expectedEvent := createFileParseStartedEvent(instants[0].Instant)
	importFileAggregate := setupAggregate(instants, 1)

	runAggregateStatesCommandTest(
		t,
		messagePrefix,
		command,
		expectedEvent,
		[]State{Imported},
		importFileAggregate,
	)

	runNotRegisteredCommandTest(
		t,
		messagePrefix,
		command,
		importFileAggregate,
	)

	runStateFailureCommandTest(
		t,
		command,
		importFileAggregate,
		[]stateFailureTestCase{
			{testName: messagePrefix + " fails if state is ParsingStarted", state: ParsingStarted},
			{testName: messagePrefix + " fails if state is ParsingRestarted", state: ParsingRestarted},
			{testName: messagePrefix + " fails if state is ParsingEnded", state: ParsingEnded},
			{testName: messagePrefix + " fails if state is ParsingFailed", state: ParsingFailed},
		},
	)
}

func TestImportFile_HandleCommand_RestartFileParse(t *testing.T) {
	t.Parallel()

	messagePrefix := "restart file parse"
	instants := []clock.Instant{
		{messagePrefix, time.Now()},
	}

	command := createRestartFileParseCommand()
	expectedEvent := createFileParseRestartedEvent(instants[0].Instant)
	importFileAggregate := setupAggregate(instants, 1)

	runAggregateStatesCommandTest(
		t,
		messagePrefix,
		command,
		expectedEvent,
		[]State{ParsingFailed},
		importFileAggregate,
	)

	runNotRegisteredCommandTest(
		t,
		messagePrefix,
		command,
		importFileAggregate,
	)

	runStateFailureCommandTest(
		t,
		command,
		importFileAggregate,
		[]stateFailureTestCase{
			{testName: messagePrefix + " fails if state is Imported", state: Imported},
			{testName: messagePrefix + " fails if state is ParsingStarted", state: ParsingStarted},
			{testName: messagePrefix + " fails if state is ParsingRestarted", state: ParsingRestarted},
			{testName: messagePrefix + " fails if state is ParsingEnded", state: ParsingEnded},
		},
	)
}

func TestImportFile_HandleCommand_EndFileParse(t *testing.T) {
	t.Parallel()

	messagePrefix := "end file parse"
	instants := []clock.Instant{
		{messagePrefix, time.Now()},
	}

	command := createEndFileParseCommand()
	expectedEvent := createFileParseEndedEvent(instants[0].Instant)
	importFileAggregate := setupAggregate(instants, 1)

	runAggregateStatesCommandTest(
		t,
		messagePrefix,
		command,
		expectedEvent,
		[]State{ParsingStarted, ParsingRestarted},
		importFileAggregate,
	)

	runNotRegisteredCommandTest(
		t,
		messagePrefix,
		command,
		importFileAggregate,
	)

	runStateFailureCommandTest(
		t,
		command,
		importFileAggregate,
		[]stateFailureTestCase{
			{testName: messagePrefix + " fails if state is Imported", state: Imported},
			{testName: messagePrefix + " fails if state is ParsingEnded", state: ParsingEnded},
			{testName: messagePrefix + " fails if state is ParsingFailed", state: ParsingFailed},
		},
	)
}

func TestImportFile_HandleCommand_FailFileParse(t *testing.T) {
	t.Parallel()

	messagePrefix := "fail file parse"
	instants := []clock.Instant{
		{messagePrefix, time.Now()},
	}

	command := createFailFileParseCommand()
	expectedEvent := createFileParseFailedEvent(instants[0].Instant)
	importFileAggregate := setupAggregate(instants, 1)

	runAggregateStatesCommandTest(
		t,
		messagePrefix,
		command,
		expectedEvent,
		[]State{ParsingStarted, ParsingRestarted},
		importFileAggregate,
	)

	runNotRegisteredCommandTest(
		t,
		messagePrefix,
		command,
		importFileAggregate,
	)

	runStateFailureCommandTest(
		t,
		command,
		importFileAggregate,
		[]stateFailureTestCase{
			{testName: messagePrefix + " fails if state is Imported", state: Imported},
			{testName: messagePrefix + " fails if state is ParsingEnded", state: ParsingEnded},
			{testName: messagePrefix + " fails if state is ParsingFailed", state: ParsingFailed},
		},
	)
}

func TestImportFile_HandleCommand_AddFileDataRow(t *testing.T) {
	t.Parallel()

	messagePrefix := "add file data row"
	instants := []clock.Instant{
		{messagePrefix, time.Now()},
	}

	rawData := setupRawData()

	dataRowId, err := setupDataRowId(rawData)
	require.NoError(t, err)

	date := time.Now()
	command, err := createAddFileDataRowCommand(date, rawData, dataRowId)
	expectedEvent := createFileDataRowAddedEvent(date, instants[0].Instant, rawData, dataRowId)
	importFileAggregate := setupAggregate(instants, 1)

	runAggregateStatesCommandTest(
		t,
		messagePrefix,
		command,
		expectedEvent,
		[]State{ParsingEnded},
		importFileAggregate,
	)

	runNotRegisteredCommandTest(
		t,
		messagePrefix,
		command,
		importFileAggregate,
	)

	runStateFailureCommandTest(
		t,
		command,
		importFileAggregate,
		[]stateFailureTestCase{
			{testName: messagePrefix + " fails if state is Imported", state: Imported},
			{testName: messagePrefix + " fails if state is Imported", state: ParsingStarted},
			{testName: messagePrefix + " fails if state is Imported", state: ParsingRestarted},
			{testName: messagePrefix + " fails if state is ParsingFailed", state: ParsingFailed},
		},
	)

	importFileAggregate.state = ParsingEnded
	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)
	importFileAggregate.importedRowStates[*dataRowId] = Verified

	t.Run(
		messagePrefix+" fails, because file data row was already added",
		func(t *testing.T) {
			importFileAggregate.SetAggregateVersion(1)

			err := importFileAggregate.HandleCommand(context.Background(), command)
			assert.Error(t, err)
			assert.Equal(
				t,
				fmt.Sprintf(
					"importFile: data row has already been added. DataRowId: %s",
					dataRowId.String(),
				),
				err.Error(),
			)
		},
	)
}

func TestImportFile_HandleCommand_VerifyFileDataRow(t *testing.T) {
	t.Parallel()

	messagePrefix := "verify file data row"
	instants := []clock.Instant{
		{messagePrefix, time.Now()},
	}

	dataRowId, err := setupDataRowId(setupRawData())
	require.NoError(t, err)

	command, err := createVerifyFileDataRowCommand(dataRowId)
	expectedEvent := createFileDataRowMarkedAsVerifiedEvent(instants[0].Instant, dataRowId)
	importFileAggregate := setupAggregate(instants, 1)

	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)
	importFileAggregate.importedRowStates[*dataRowId] = Unverified

	runAggregateStatesCommandTest(
		t,
		messagePrefix,
		command,
		expectedEvent,
		[]State{ParsingEnded},
		importFileAggregate,
	)

	runNotRegisteredCommandTest(
		t,
		messagePrefix,
		command,
		importFileAggregate,
	)

	runStateFailureCommandTest(
		t,
		command,
		importFileAggregate,
		[]stateFailureTestCase{
			{testName: messagePrefix + " fails if state is Imported", state: Imported},
			{testName: messagePrefix + " fails if state is Imported", state: ParsingStarted},
			{testName: messagePrefix + " fails if state is Imported", state: ParsingRestarted},
			{testName: messagePrefix + " fails if state is ParsingFailed", state: ParsingFailed},
		},
	)

	importFileAggregate.state = ParsingEnded
	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)
	t.Run(
		messagePrefix+" fails, because file data row was already added",
		func(t *testing.T) {
			importFileAggregate.SetAggregateVersion(1)

			err := importFileAggregate.HandleCommand(context.Background(), command)
			assert.Error(t, err)
			assert.Equal(
				t,
				fmt.Sprintf(
					"importFile: data row was not imported or is in an invalid state for verification. DataRowId: %s, Exists: %t, State: %s",
					dataRowId.String(),
					false,
					"",
				),
				err.Error(),
			)
		},
	)

	for _, dataRowState := range []DataRowState{Verified, Invalid} {
		importFileAggregate.importedRowStates[*dataRowId] = dataRowState

		t.Run(
			messagePrefix+" fails, because file data row was already added",
			func(t *testing.T) {
				importFileAggregate.SetAggregateVersion(1)

				err := importFileAggregate.HandleCommand(context.Background(), command)
				assert.Error(t, err)
				assert.Equal(
					t,
					fmt.Sprintf(
						"importFile: data row was not imported or is in an invalid state for verification. DataRowId: %s, Exists: %t, State: %s",
						dataRowId.String(),
						true,
						dataRowState,
					),
					err.Error(),
				)
			},
		)
	}
}

func TestImportFile_HandleCommand_InvalidateFileDataRow(t *testing.T) {
	t.Parallel()

	messagePrefix := "invalidate file data row"
	instants := []clock.Instant{
		{messagePrefix, time.Now()},
	}

	dataRowId, err := setupDataRowId(setupRawData())
	require.NoError(t, err)

	command, err := createInvalidateFileDataRowCommand(dataRowId)
	expectedEvent := createFileDataRowMarkedAsInvalidEvent(instants[0].Instant, dataRowId)
	importFileAggregate := setupAggregate(instants, 1)

	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)
	importFileAggregate.importedRowStates[*dataRowId] = Unverified

	runAggregateStatesCommandTest(
		t,
		messagePrefix,
		command,
		expectedEvent,
		[]State{ParsingEnded},
		importFileAggregate,
	)

	runNotRegisteredCommandTest(
		t,
		messagePrefix,
		command,
		importFileAggregate,
	)

	runStateFailureCommandTest(
		t,
		command,
		importFileAggregate,
		[]stateFailureTestCase{
			{testName: messagePrefix + " fails if state is Imported", state: Imported},
			{testName: messagePrefix + " fails if state is Imported", state: ParsingStarted},
			{testName: messagePrefix + " fails if state is Imported", state: ParsingRestarted},
			{testName: messagePrefix + " fails if state is ParsingFailed", state: ParsingFailed},
		},
	)

	importFileAggregate.state = ParsingEnded
	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)
	t.Run(
		messagePrefix+" fails, because file data row was already added",
		func(t *testing.T) {
			importFileAggregate.SetAggregateVersion(1)

			err := importFileAggregate.HandleCommand(context.Background(), command)
			assert.Error(t, err)
			assert.Equal(
				t,
				fmt.Sprintf(
					"importFile: data row was not imported or is in an invalid state for invalidation. DataRowId: %s, Exists: %t, State: %s",
					dataRowId.String(),
					false,
					"",
				),
				err.Error(),
			)
		},
	)

	for _, dataRowState := range []DataRowState{Verified, Invalid} {
		importFileAggregate.importedRowStates[*dataRowId] = dataRowState
		t.Run(
			messagePrefix+" fails, because file data row was "+string(dataRowState),
			func(t *testing.T) {
				importFileAggregate.SetAggregateVersion(1)

				err := importFileAggregate.HandleCommand(context.Background(), command)
				assert.Error(t, err)
				assert.Equal(
					t,
					fmt.Sprintf(
						"importFile: data row was not imported or is in an invalid state for invalidation. DataRowId: %s, Exists: %t, State: %s",
						dataRowId.String(),
						true,
						dataRowState,
					),
					err.Error(),
				)
			},
		)
	}
}

func TestImportFile_HandleCommand_RegisterAccountMovementIdForVerifiedFileDataRow(t *testing.T) {
	t.Parallel()

	messagePrefix := "invalidate file data row"
	instants := []clock.Instant{
		{messagePrefix, time.Now()},
	}

	dataRowId, err := setupDataRowId(setupRawData())
	require.NoError(t, err)

	command, err := createRegisterAccountMovementIdForVerifiedFileDataRowCommand(dataRowId)
	expectedEvent := createAccountMovementIdForVerifiedFileDataRowRegisteredEvent(instants[0].Instant, dataRowId)
	importFileAggregate := setupAggregate(instants, 1)

	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)
	importFileAggregate.importedRowStates[*dataRowId] = Verified

	runAggregateStatesCommandTest(
		t,
		messagePrefix,
		command,
		expectedEvent,
		[]State{ParsingEnded},
		importFileAggregate,
	)

	runNotRegisteredCommandTest(
		t,
		messagePrefix,
		command,
		importFileAggregate,
	)

	runStateFailureCommandTest(
		t,
		command,
		importFileAggregate,
		[]stateFailureTestCase{
			{testName: messagePrefix + " fails if state is Imported", state: Imported},
			{testName: messagePrefix + " fails if state is Imported", state: ParsingStarted},
			{testName: messagePrefix + " fails if state is Imported", state: ParsingRestarted},
			{testName: messagePrefix + " fails if state is ParsingFailed", state: ParsingFailed},
		},
	)

	importFileAggregate.state = ParsingEnded
	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)
	t.Run(
		messagePrefix+" fails, because file data row was already added",
		func(t *testing.T) {
			importFileAggregate.SetAggregateVersion(1)

			err := importFileAggregate.HandleCommand(context.Background(), command)
			assert.Error(t, err)
			assert.Equal(
				t,
				fmt.Sprintf(
					"importFile: data row was not imported or is in an invalid state for verification account movement. DataRowId: %s, Exists: %t, State: %s",
					dataRowId.String(),
					false,
					"",
				),
				err.Error(),
			)
		},
	)

	for _, dataRowState := range []DataRowState{Unverified, Invalid} {
		importFileAggregate.importedRowStates[*dataRowId] = dataRowState
		t.Run(
			messagePrefix+" fails, because file data row was "+string(dataRowState),
			func(t *testing.T) {
				importFileAggregate.SetAggregateVersion(1)

				err := importFileAggregate.HandleCommand(context.Background(), command)
				assert.Error(t, err)
				assert.Equal(
					t,
					fmt.Sprintf(
						"importFile: data row was not imported or is in an invalid state for verification account movement. DataRowId: %s, Exists: %t, State: %s",
						dataRowId.String(),
						true,
						dataRowState,
					),
					err.Error(),
				)
			},
		)
	}
}

func TestImportFile_ApplyEvent_NewImportFileRegistered(t *testing.T) {
	t.Parallel()

	importFileAggregate := setupAggregate([]clock.Instant{}, 1)

	event := createNewImportFileRegisteredEvent(time.Now())

	err := importFileAggregate.ApplyEvent(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, Imported, importFileAggregate.state)
	assert.Len(t, importFileAggregate.importedRowStates, 0)
}

func TestImportFile_ApplyEvent_FileParseStarted(t *testing.T) {
	t.Parallel()

	importFileAggregate := setupAggregate([]clock.Instant{}, 1)

	event := createFileParseStartedEvent(time.Now())

	err := importFileAggregate.ApplyEvent(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, ParsingStarted, importFileAggregate.state)
	assert.Len(t, importFileAggregate.importedRowStates, 0)
}

func TestImportFile_ApplyEvent_FileParseRestarted(t *testing.T) {
	t.Parallel()

	importFileAggregate := setupAggregate([]clock.Instant{}, 1)

	event := createFileParseRestartedEvent(time.Now())

	err := importFileAggregate.ApplyEvent(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, ParsingRestarted, importFileAggregate.state)
	assert.Len(t, importFileAggregate.importedRowStates, 0)
}

func TestImportFile_ApplyEvent_FileParseEnded(t *testing.T) {
	t.Parallel()

	importFileAggregate := setupAggregate([]clock.Instant{}, 1)

	event := createFileParseEndedEvent(time.Now())

	err := importFileAggregate.ApplyEvent(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, ParsingEnded, importFileAggregate.state)
	assert.Len(t, importFileAggregate.importedRowStates, 0)
}

func TestImportFile_ApplyEvent_FileParseFailed(t *testing.T) {
	t.Parallel()

	importFileAggregate := setupAggregate([]clock.Instant{}, 1)

	event := createFileParseFailedEvent(time.Now())

	err := importFileAggregate.ApplyEvent(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, ParsingFailed, importFileAggregate.state)
	assert.Len(t, importFileAggregate.importedRowStates, 0)
}

func TestImportFile_ApplyEvent_FileDataRowAdded(t *testing.T) {
	t.Parallel()

	importFileAggregate := setupAggregate([]clock.Instant{}, 1)
	importFileAggregate.state = ParsingEnded
	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)

	dateRowId, err := setupDataRowId(setupRawData())
	require.NoError(t, err)

	event := createFileDataRowAddedEvent(time.Now(), time.Now(), setupRawData(), dateRowId)

	err = importFileAggregate.ApplyEvent(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, ParsingEnded, importFileAggregate.state)
	assert.Len(t, importFileAggregate.importedRowStates, 1)
	assert.Equal(t, Unverified, importFileAggregate.importedRowStates[*dateRowId])
}

func TestImportFile_ApplyEvent_FileDataRowMarkedAsVerified(t *testing.T) {
	t.Parallel()

	importFileAggregate := setupAggregate([]clock.Instant{}, 1)
	importFileAggregate.state = ParsingEnded
	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)

	dateRowId, err := setupDataRowId(setupRawData())
	require.NoError(t, err)

	event := createFileDataRowMarkedAsVerifiedEvent(time.Now(), dateRowId)

	err = importFileAggregate.ApplyEvent(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, ParsingEnded, importFileAggregate.state)
	assert.Len(t, importFileAggregate.importedRowStates, 1)
	assert.Equal(t, Verified, importFileAggregate.importedRowStates[*dateRowId])
}

func TestImportFile_ApplyEvent_FileDataRowMarkedAsInvalid(t *testing.T) {
	t.Parallel()

	importFileAggregate := setupAggregate([]clock.Instant{}, 1)
	importFileAggregate.state = ParsingEnded
	importFileAggregate.importedRowStates = make(map[DataRowId]DataRowState)

	dateRowId, err := setupDataRowId(setupRawData())
	require.NoError(t, err)

	event := createFileDataRowMarkedAsInvalidEvent(time.Now(), dateRowId)

	err = importFileAggregate.ApplyEvent(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, ParsingEnded, importFileAggregate.state)
	assert.Len(t, importFileAggregate.importedRowStates, 1)
	assert.Equal(t, Invalid, importFileAggregate.importedRowStates[*dateRowId])
}

func runAggregateStatesCommandTest(
	t *testing.T,
	messagePrefix string,
	command eventhorizon.Command,
	expectedEvent eventhorizon.Event,
	aggregateStates []State,
	importFileAggregate *ImportFile,
) {
	asserts := assert.New(t)

	for _, state := range aggregateStates {
		t.Run(
			fmt.Sprintf("%s success for state %s", messagePrefix, state),
			func(t *testing.T) {
				importFileAggregate.state = state

				err := importFileAggregate.HandleCommand(context.Background(), command)
				require.NoError(t, err)

				uncommittedEvents := importFileAggregate.UncommittedEvents()
				asserts.Equal(1, len(uncommittedEvents))
				asserts.Equal(expectedEvent, uncommittedEvents[0])
				asserts.Equal(expectedEvent.EventType(), uncommittedEvents[0].EventType())
				asserts.Equal(expectedEvent.AggregateType(), uncommittedEvents[0].AggregateType())
				asserts.Equal(expectedEvent.Data(), uncommittedEvents[0].Data())

				importFileAggregate.ClearUncommittedEvents()
			},
		)
	}
}

func runNotRegisteredCommandTest(
	t *testing.T,
	messagePrefix string,
	command eventhorizon.Command,
	importFileAggregate *ImportFile,
) {
	t.Run(
		messagePrefix+" fails, because import file is not registered",
		func(t *testing.T) {
			importFileAggregate.SetAggregateVersion(0)

			err := importFileAggregate.HandleCommand(context.Background(), command)
			assert.Error(t, err)
		},
	)
}

func runStateFailureCommandTest(
	t *testing.T,
	command eventhorizon.Command,
	importFileAggregate *ImportFile,
	stateFailureTestCases []stateFailureTestCase,
) {
	asserts := assert.New(t)

	for _, testCase := range stateFailureTestCases {
		t.Run(
			testCase.testName,
			func(t *testing.T) {
				importFileAggregate.SetAggregateVersion(1)
				importFileAggregate.state = testCase.state

				err := importFileAggregate.HandleCommand(context.Background(), command)
				asserts.Error(err)
			},
		)
	}

}

func createRegisterNewImportFileCommand() eventhorizon.Command {
	return &RegisterNewImportFile{
		ImportFileId: *setupImportFileId(),
		AccountId:    *setupAccountId(),
		Filename:     "a-file.csv",
		FileType:     CSV,
	}
}

func createStartFileParseCommand() eventhorizon.Command {
	return &StartFileParse{
		ImportFileId: *setupImportFileId(),
	}
}

func createRestartFileParseCommand() eventhorizon.Command {
	return &RestartFileParse{
		ImportFileId: *setupImportFileId(),
	}
}

func createEndFileParseCommand() eventhorizon.Command {
	return &EndFileParse{
		ImportFileId: *setupImportFileId(),
	}
}

func createFailFileParseCommand() eventhorizon.Command {
	return &FailFileParse{
		ImportFileId: *setupImportFileId(),
	}
}

func createAddFileDataRowCommand(
	date time.Time,
	rawData map[string]any,
	dataRowId *DataRowId,
) (eventhorizon.Command, error) {
	return &AddFileDataRow{
		ImportFileId:  *setupImportFileId(),
		FileDataRowId: *dataRowId,
		Date:          date,
		Description:   "a row",
		Amount:        10000,
		RawData:       rawData,
	}, nil
}

func createVerifyFileDataRowCommand(dataRowId *DataRowId) (eventhorizon.Command, error) {
	return &VerifyFileDataRow{
		ImportFileId:    *setupImportFileId(),
		FileDataRowId:   *dataRowId,
		MovementTypeId:  nil,
		SourceAccountId: nil,
		Description:     "a row",
		TagIds:          []*tagcategory.TagId{setupTagId()},
	}, nil
}

func createInvalidateFileDataRowCommand(dataRowId *DataRowId) (eventhorizon.Command, error) {
	return &InvalidateFileDataRow{
		ImportFileId:  *setupImportFileId(),
		FileDataRowId: *dataRowId,
		Reason:        "a reason",
	}, nil
}

func createRegisterAccountMovementIdForVerifiedFileDataRowCommand(dataRowId *DataRowId) (eventhorizon.Command, error) {
	return &RegisterAccountMovementIdForVerifiedFileDataRow{
		ImportFileId:      *setupImportFileId(),
		FileDataRowId:     *dataRowId,
		AccountMovementId: *setupAccountMovementId(),
	}, nil
}

func createNewImportFileRegisteredEvent(createdAt time.Time) eventhorizon.Event {
	return eventhorizon.NewEvent(
		NewImportFileRegistered,
		&NewImportFileRegisteredData{
			ImportFileId: setupImportFileId(),
			AccountId:    setupAccountId(),
			Filename:     "a-file.csv",
			FileType:     CSV,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 1),
	)
}

func createFileParseStartedEvent(createdAt time.Time) eventhorizon.Event {
	return eventhorizon.NewEvent(
		FileParseStarted,
		&FileParseStartedData{
			ImportFileId: setupImportFileId(),
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 2),
	)
}

func createFileParseRestartedEvent(createdAt time.Time) eventhorizon.Event {
	return eventhorizon.NewEvent(
		FileParseRestarted,
		&FileParseRestartedData{
			ImportFileId: setupImportFileId(),
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 2),
	)
}

func createFileParseEndedEvent(createdAt time.Time) eventhorizon.Event {
	return eventhorizon.NewEvent(
		FileParseEnded,
		&FileParseEndedData{
			ImportFileId: setupImportFileId(),
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 2),
	)
}

func createFileParseFailedEvent(createdAt time.Time) eventhorizon.Event {
	return eventhorizon.NewEvent(
		FileParseFailed,
		&FileParseFailedData{
			ImportFileId: setupImportFileId(),
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 2),
	)
}

func createFileDataRowAddedEvent(
	date time.Time,
	createdAt time.Time,
	rawData map[string]any,
	dataRowId *DataRowId,
) eventhorizon.Event {
	return eventhorizon.NewEvent(
		FileDataRowAdded,
		&FileDataRowAddedData{
			ImportFileId:  setupImportFileId(),
			FileDataRowId: dataRowId,
			Date:          date,
			Description:   "a row",
			Amount:        10000,
			RawData:       rawData,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 2),
	)
}

func createFileDataRowMarkedAsVerifiedEvent(
	createdAt time.Time,
	dataRowId *DataRowId,
) eventhorizon.Event {
	return eventhorizon.NewEvent(
		FileDataRowMarkedAsVerified,
		&FileDataRowMarkedAsVerifiedData{
			ImportFileId:    setupImportFileId(),
			FileDataRowId:   dataRowId,
			Description:     "a row",
			MovementTypeId:  nil,
			SourceAccountId: nil,
			TagIds:          []*tagcategory.TagId{setupTagId()},
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 2),
	)
}

func createFileDataRowMarkedAsInvalidEvent(
	createdAt time.Time,
	dataRowId *DataRowId,
) eventhorizon.Event {
	return eventhorizon.NewEvent(
		FileDataRowMarkedAsInvalid,
		&FileDataRowMarkedAsInvalidData{
			ImportFileId:  setupImportFileId(),
			FileDataRowId: dataRowId,
			Reason:        "a reason",
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 2),
	)
}

func createAccountMovementIdForVerifiedFileDataRowRegisteredEvent(
	createdAt time.Time,
	dataRowId *DataRowId,
) eventhorizon.Event {
	return eventhorizon.NewEvent(
		AccountMovementIdForVerifiedFileDataRowRegistered,
		&AccountMovementIdForVerifiedFileDataRowRegisteredData{
			ImportFileId:      setupImportFileId(),
			FileDataRowId:     dataRowId,
			AccountMovementId: setupAccountMovementId(),
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupImportFileId(), 2),
	)
}
