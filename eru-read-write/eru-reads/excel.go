package eru_reads

import (
	"bytes"
	"context"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/xuri/excelize/v2"
)

type ExcelReadData struct {
	ReadData
	SheetNames []string
}

func (erd *ExcelReadData) ReadAsJson(ctx context.Context, readData []byte) (readOutput map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("WriteColumnar - Start")
	f, err := excelize.OpenReader(bytes.NewReader(readData))
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	defer func() {
		// Close the spreadsheet.
		if err := f.Close(); err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
	}()
	rows, err := f.GetRows("Orders")
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	for _, row := range rows {
		for _, colCell := range row {
			fmt.Print(colCell, "\t")
		}
		fmt.Println()
	}
	return nil, nil
}
