package eru_reads

import (
	"context"
	"errors"
)

type ReadData struct {
	ColumnarDataHeader         []string
	ColumnarDataHeaderFirstRow bool
}

type ReadI interface {
	ReadAsJson(ctx context.Context, readData []byte) (readOutput map[string]interface{}, err error)
}

func (rd *ReadData) ReadAsJson(ctx context.Context, readData []byte) (readOutput map[string]interface{}, err error) {
	return nil, errors.New("ReadColumnar method not supported")
}
