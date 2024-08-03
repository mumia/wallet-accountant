package bankfileparser

import (
	"bytes"
	"context"
	"encoding/csv"
	"github.com/Clever/csvlint"
	"golang.org/x/text/encoding/charmap"
	"io"
	"time"
	"walletaccountant/account"
	"walletaccountant/importfile"
)

var _ BankFileParser = &BcpCSVParser{}

type BcpCSVParser struct {
}

func NewBcpCSVParser() *BcpCSVParser {
	return &BcpCSVParser{}
}

func (bcpCSVParser *BcpCSVParser) Id() string {
	return ParserId(bcpCSVParser.BankName(), bcpCSVParser.FileType())
}

func (bcpCSVParser *BcpCSVParser) BankName() account.BankName {
	return account.BCP
}

func (bcpCSVParser *BcpCSVParser) FileType() importfile.FileType {
	return importfile.CSV
}

func (bcpCSVParser *BcpCSVParser) CleanAndValidate(file io.Reader) (*bytes.Buffer, interface{}, error) {
	buffer, err := TrimLinesAndCleanupCSV(file, 8, 3, charmap.ISO8859_15)
	if err != nil {
		return nil, nil, err
	}

	csvErrors, _, err := csvlint.Validate(file, ';', true)
	if err != nil {
		return nil, nil, err
	}

	if len(csvErrors) > 0 {
		return nil, csvErrors, nil
	}

	return buffer, nil, nil
}

func (bcpCSVParser *BcpCSVParser) Parse(
	ctx context.Context,
	file io.Reader,
	importFileId *importfile.Id,
	accountId *account.Id,
	rowHandler BankFileDataRowHandler,
) error {
	csvReader := csv.NewReader(file)

	var headers []string = nil
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if headers == nil {
			headers = record

			continue
		}

		err = prepareAndCallRowHandler(
			ctx,
			importFileId,
			accountId,
			bcpCSVParser.readRowFromRecord,
			rowHandler,
			headers,
			record,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (bcpCSVParser *BcpCSVParser) readRowFromRecord(
	headers []string,
	row []string,
) (*time.Time, *string, *int64, *map[string]any, error) {
	// Date 22-01-2024
	rowDate, err := time.Parse("02-01-2006", row[1])
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Description
	rowDescription := row[2]

	// Amount
	rowAmount, err := parseAmount(row[3])
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Raw data
	data := make(map[string]any)
	for i, header := range headers {
		data[header] = row[i]
	}

	return &rowDate, &rowDescription, &rowAmount, &data, nil
}
