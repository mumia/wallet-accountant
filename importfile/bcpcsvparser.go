package importfile

import (
	"encoding/csv"
	"fmt"
	"github.com/Clever/csvlint"
	"io"
	"os"
	"walletaccountant/account"
	"walletaccountant/importfile/bankfilereader"
)

var _ bankfilereader.BankFileParser = &BcpCSVParser{}

type BcpCSVParser struct {
}

func NewBcpCSVParser() *BcpCSVParser {
	return &BcpCSVParser{}
}

func (n26CSVParser *BcpCSVParser) Id() string {
	return bankfilereader.BankFileParserId(n26CSVParser.BankName(), n26CSVParser.FileType())
}

func (n26CSVParser *BcpCSVParser) BankName() account.BankName {
	return account.BCP
}

func (n26CSVParser *BcpCSVParser) FileType() bankfilereader.FileType {
	return bankfilereader.CSV
}

func (n26CSVParser *BcpCSVParser) CleanAndValidate(file *os.File) (interface{}, error) {
	file, err := bankfilereader.RemoveStartLinesFromCSV(file, 4)
	if err != nil {
		return nil, err
	}

	csvErrors, _, err := csvlint.Validate(file, ',', true)
	if err != nil {
		return nil, err
	}

	if len(csvErrors) > 0 {
		return csvErrors, nil
	}

	return nil, nil
}

func (n26CSVParser *BcpCSVParser) Parse(file io.Reader, handler bankfilereader.BankFileDataRowHandler) error {
	csvReader := csv.NewReader(file)

	var header []string = nil
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if header == nil {
			header = record

			continue
		}

		fmt.Println(record)

		//Call handler
	}

	return nil
}
