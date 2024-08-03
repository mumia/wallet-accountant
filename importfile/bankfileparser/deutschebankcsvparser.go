package bankfileparser

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"github.com/Clever/csvlint"
	"golang.org/x/text/encoding/charmap"
	"io"
	"time"
	"walletaccountant/account"
	"walletaccountant/importfile"
)

var _ BankFileParser = &DeutscheBankCSVParser{}

type DeutscheBankCSVParser struct {
}

func NewDeutscheBankCSVParser() *DeutscheBankCSVParser {
	return &DeutscheBankCSVParser{}
}

func (dbCSVParser *DeutscheBankCSVParser) Id() string {
	return ParserId(dbCSVParser.BankName(), dbCSVParser.FileType())
}

func (dbCSVParser *DeutscheBankCSVParser) BankName() account.BankName {
	return account.DB
}

func (dbCSVParser *DeutscheBankCSVParser) FileType() importfile.FileType {
	return importfile.CSV
}

func (dbCSVParser *DeutscheBankCSVParser) CleanAndValidate(file io.Reader) (*bytes.Buffer, interface{}, error) {
	buffer, err := TrimLinesAndCleanupCSV(file, 4, 2, charmap.ISO8859_1)
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

func (dbCSVParser *DeutscheBankCSVParser) Parse(
	ctx context.Context,
	file io.Reader,
	importFileId *importfile.Id,
	accountId *account.Id,
	rowHandler BankFileDataRowHandler,
) error {
	reader := csv.NewReader(file)
	reader.Comma = ';'

	var headers []string = nil
	for {
		record, err := reader.Read()
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
			dbCSVParser.readRowFromRecord,
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

func (dbCSVParser *DeutscheBankCSVParser) readRowFromRecord(
	headers []string,
	row []string,
) (*time.Time, *string, *int64, *map[string]any, error) {
	// Date 01/29/2024
	rowDate, err := time.Parse("01/02/2006", row[1])
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Description
	rowDescription := row[3]

	// Amount
	amountString := ""
	if row[16] != "" {
		amountString = row[16]
	} else {
		amountString = row[17]
	}
	if amountString == "" {
		return nil, nil, nil, nil, fmt.Errorf(
			"failed to find a proper amount for Debit(%s) or Credit(%s)",
			row[16],
			row[17],
		)
	}
	rowAmount, err := parseAmount(amountString)
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
