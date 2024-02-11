package bankfilereader

import (
	"bufio"
	"bytes"
	"go.uber.org/fx"
	"io"
	"os"
	"walletaccountant/account"
)

type BankFileParser interface {
	Id() string
	BankName() account.BankName
	FileType() FileType
	CleanAndValidate(file *os.File) (interface{}, error)
	Parse(file io.Reader, handler BankFileDataRowHandler) error
}

func AsBankFileParser(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(BankFileParser)),
		fx.ResultTags(`group:"bankFileParsers"`),
	)
}

func BankFileParserId(bankName account.BankName, fileType FileType) string {
	return string(bankName) + "_" + string(fileType)
}

func RemoveStartLinesFromCSV(file *os.File, linesToRemove int) (*os.File, error) {
	scanner := bufio.NewScanner(file)
	var bufferBytes []byte
	buffer := bytes.NewBuffer(bufferBytes)

	var text string
	count := 0
	for scanner.Scan() {
		text = scanner.Text()

		if count < linesToRemove {
			continue
		}

		_, err := buffer.WriteString(text + "\n")
		if err != nil {
			return nil, err
		}

		count++
	}

	err := file.Truncate(0)
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	_, err = buffer.WriteTo(file)
	if err != nil {
		return nil, err
	}

	return file, nil
}
