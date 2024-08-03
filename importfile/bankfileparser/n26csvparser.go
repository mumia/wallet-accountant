package bankfileparser

import (
	"bytes"
	"context"
	"encoding/csv"
	"github.com/Clever/csvlint"
	"io"
	"time"
	"walletaccountant/account"
	"walletaccountant/importfile"
)

var _ BankFileParser = &N26CSVParser{}

type N26CSVParser struct {
}

func NewN26CSVParser() *N26CSVParser {
	return &N26CSVParser{}
}

func (n26CSVParser *N26CSVParser) Id() string {
	return ParserId(n26CSVParser.BankName(), n26CSVParser.FileType())
}

func (n26CSVParser *N26CSVParser) BankName() account.BankName {
	return account.N26
}

func (n26CSVParser *N26CSVParser) FileType() importfile.FileType {
	return importfile.CSV
}

func (n26CSVParser *N26CSVParser) CleanAndValidate(reader io.Reader) (*bytes.Buffer, interface{}, error) {
	buffer, err := TrimLinesAndCleanupCSV(reader, 0, 0, nil)
	if err != nil {
		return nil, nil, err
	}

	csvErrors, _, err := csvlint.Validate(reader, ',', true)
	if err != nil {
		return nil, nil, err
	}

	if len(csvErrors) > 0 {
		return nil, csvErrors, nil
	}

	return buffer, nil, nil
}

func (n26CSVParser *N26CSVParser) Parse(
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
			n26CSVParser.readRowFromRecord,
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

func (n26CSVParser *N26CSVParser) readRowFromRecord(
	headers []string,
	row []string,
) (*time.Time, *string, *int64, *map[string]any, error) {
	// Date 2024-01-25
	rowDate, err := time.Parse("2006-01-02", row[0])
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Description
	rowDescription := row[1]

	// Amount
	rowAmount, err := parseAmount(row[5])
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
