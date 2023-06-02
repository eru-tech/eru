package eru_reads

import (
	"context"
	"errors"
)

type ReadData struct {
}

type FileReadData struct {
	HeaderRow     int      `json:"header_row"`
	DataStartRow  int      `json:"data_start_row"`
	ColumnHeaders []string `json:"column_headers"`
	Columns       []int    `json:"columns"`
}

type ReadI interface {
	ReadAsJson(ctx context.Context, readData []byte) (readOutput map[string]interface{}, err error)
}

func (rd *ReadData) ReadAsJson(ctx context.Context, readData []byte) (readOutput map[string]interface{}, err error) {
	return nil, errors.New("ReadColumnar method not supported")
}
