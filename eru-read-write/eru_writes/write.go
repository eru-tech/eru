package eru_writes

import "errors"

const (
	OutputTypeExcel = "excel"
	OutputTypeCsv   = "csv"
)

type WriteData struct {
	ColumnarDataMap            map[string][][]interface{}
	ColumnarData               [][]interface{}
	ColumnarDataHeader         []string
	ColumnarDataHeaderFirstRow bool
	FileName                   string
}

type WriteI interface {
	WriteColumnar() (writeOutput []byte, err error)
}

func (wd *WriteData) WriteColumnar() (writeOutput []byte, err error) {
	return nil, errors.New("WriteColumnar method not supported")
}
