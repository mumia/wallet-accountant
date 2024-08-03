package bankfileparser

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/shopspring/decimal"
	"go.uber.org/fx"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
	"io"
	"strings"
	"time"
	"walletaccountant/account"
	"walletaccountant/importfile"
	"walletaccountant/importfile/bankfilereadererror"
)

type readRowFromRecord = func(
	headers []string,
	row []string,
) (*time.Time, *string, *int64, *map[string]any, error)

type BankFileDataRowHandler func(
	ctx context.Context,
	importFileId *importfile.Id,
	row *BankFileDataRow,
) *bankfilereadererror.ReaderError

type rowReader interface {
	readRowFromRecord
}

type BankFileDataRow struct {
	ImportFileId *importfile.Id
	AccountId    *account.Id
	DataRowId    *importfile.DataRowId
	Date         time.Time
	Description  string
	Amount       int64
	RawData      map[string]interface{}
}

type BankFileParser interface {
	Id() string
	BankName() account.BankName
	FileType() importfile.FileType
	CleanAndValidate(reader io.Reader) (*bytes.Buffer, interface{}, error)
	Parse(
		ctx context.Context,
		file io.Reader,
		importFileId *importfile.Id,
		accountId *account.Id,
		rowHandler BankFileDataRowHandler,
	) error
}

func AsBankFileParser(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(BankFileParser)),
		fx.ResultTags(`group:"bankFileParsers"`),
	)
}

func ParserId(bankName account.BankName, fileType importfile.FileType) string {
	return string(bankName) + "_" + string(fileType)
}

func TrimLinesAndCleanupCSV(
	reader io.Reader,
	removeFromStart int,
	removeFromEnd int,
	encoding *charmap.Charmap,
) (*bytes.Buffer, error) {
	if encoding != nil {
		// For reference
		// https://stackoverflow.com/questions/32518432/how-to-convert-from-an-encoding-to-utf-8-in-go
		reader = transform.NewReader(reader, encoding.NewDecoder())
	}

	scanner := bufio.NewScanner(reader)
	lines := make([][]byte, 0)

	var line []byte
	count := 0
	for scanner.Scan() {
		if count < removeFromStart {
			count++

			continue
		}

		line = []byte(strings.ToValidUTF8(scanner.Text(), ""))

		if len(line) == 0 {
			count++

			continue
		}

		// Remove all NUL characters from the string (BCP files have them)
		line = bytes.ReplaceAll(line, []byte("\u0000"), []byte(""))

		lines = append(lines, line)

		count++
	}

	if len(lines) < removeFromEnd {
		return nil, errors.New("no CSV lines left after start + cleanup")
	}

	// Remove lines from end
	lines = lines[:len(lines)-removeFromEnd]

	if len(lines) < 1 {
		return nil, errors.New("no CSV lines left after start + cleanup + end")
	}

	return bytes.NewBuffer(bytes.Join(lines, []byte("\n"))), nil
}

func hashRawData(rawData *map[string]any) (uint64, error) {
	rowHash, err := hashstructure.Hash(rawData, hashstructure.FormatV2, nil)
	if err != nil {
		return 0, err
	}

	return rowHash, nil
}

func writeCleanedFile(file io.ReadWriteSeeker, buffer *bytes.Buffer) error {
	//err := file.Truncate(int64(0))
	//if err != nil {
	//	return err
	//}
	//
	//_, err = file.Seek(0, 0)
	//if err != nil {
	//	return err
	//}

	_, err := buffer.WriteTo(file)
	if err != nil {
		return err
	}

	//_, err = file.Seek(0, 0)
	//if err != nil {
	//	return err
	//}

	return nil
}

func prepareAndCallRowHandler(
	ctx context.Context,
	importFileId *importfile.Id,
	accountId *importfile.Id,
	readRow readRowFromRecord,
	rowHandler BankFileDataRowHandler,
	headers []string,
	record []string,
) error {
	rowDate, rowDescription, rowAmount, rowRawData, err := readRow(headers, record)
	if err != nil {
		return err
	}

	rowHash, err := hashRawData(rowRawData)
	if err != nil {
		return err
	}

	dataRowId, err := importfile.DataRowIdGenerate(importFileId, rowHash)
	if err != nil {
		return err
	}

	waErr := rowHandler(
		ctx,
		importFileId,
		&BankFileDataRow{
			ImportFileId: importFileId,
			AccountId:    accountId,
			DataRowId:    dataRowId,
			Date:         *rowDate,
			Description:  *rowDescription,
			Amount:       *rowAmount,
			RawData:      *rowRawData,
		},
	)
	if waErr != nil {
		return err
	}

	return nil
}

func parseAmount(stringAmount string) (int64, error) {
	value, err := decimal.NewFromString(stringAmount)
	if err != nil {
		return 0, err
	}

	return value.Mul(decimal.NewFromInt(100)).IntPart(), nil
}
