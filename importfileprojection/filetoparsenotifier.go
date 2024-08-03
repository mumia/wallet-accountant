package importfileprojection

import "walletaccountant/importfile"

type FileToParseNotifier struct {
	fileToParseChannel chan *importfile.Id
}

func NewFileToParseNotifier() *FileToParseNotifier {
	return &FileToParseNotifier{
		fileToParseChannel: make(chan *importfile.Id),
	}
}

func (notifier *FileToParseNotifier) Channel() chan *importfile.Id {
	return notifier.fileToParseChannel
}
