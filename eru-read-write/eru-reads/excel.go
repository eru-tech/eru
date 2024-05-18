package eru_reads

import (
	"bytes"
	"context"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-read-write/validator"
	"github.com/xuri/excelize/v2"
	"strconv"
	"strings"
)

type ExcelReadData struct {
	ReadData
	Sheets map[string]FileReadData `json:"sheets"`
}

func (erd *ExcelReadData) ReadAsJson(ctx context.Context, readData []byte) (readOutput map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("WriteColumnar - Start")
	logs.WithContext(ctx).Info(fmt.Sprint(erd.Sheets))
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

	if erd.Sheets == nil {
		erd.Sheets = make(map[string]FileReadData)
		for _, sn := range f.GetSheetList() {
			erd.Sheets[sn] = FileReadData{}
		}
	}
	schema := validator.Schema{}
	for sheetName, sheetObj := range erd.Sheets {
		if sheetName == "*" {
			for _, sn := range f.GetSheetList() {
				erd.Sheets[sn] = sheetObj
			}
			delete(erd.Sheets, "*")
			break
		}
	}
	for sheetName, sheetObj := range erd.Sheets {

		err = schema.SetFields(ctx, sheetObj.Fields)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}

		var sheetData []map[string]interface{}
		logs.WithContext(ctx).Info(fmt.Sprint(sheetName))
		rows, rowsErr := f.GetRows(sheetName, excelize.Options{RawCellValue: true})
		if rowsErr != nil {
			err = rowsErr
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		_ = rows
		_ = sheetObj
		logs.WithContext(ctx).Info(sheetName)

		var cols []int
		var colHeaders []string

		if sheetObj.ColumnHeaders == nil {
			for _, cellHeader := range rows[sheetObj.HeaderRow] {
				colHeaders = append(colHeaders, cellHeader)
			}
		} else if len(sheetObj.ColumnHeaders) == 0 {
			for _, cellHeader := range rows[sheetObj.HeaderRow] {
				colHeaders = append(colHeaders, cellHeader)
			}
		} else {
			for chNo, ch := range sheetObj.ColumnHeaders {
				if ch == "" {
					logs.WithContext(ctx).Info(fmt.Sprint(len(rows[sheetObj.HeaderRow]), " > ", chNo))
					if len(rows[sheetObj.HeaderRow]) > chNo {
						colHeaders = append(colHeaders, rows[sheetObj.HeaderRow][chNo])
					} else {
						colHeaders = append(colHeaders, "")
					}
				} else {
					colHeaders = append(colHeaders, ch)
				}
			}
		}
		logs.WithContext(ctx).Info(fmt.Sprint(colHeaders))

		if sheetObj.Columns == nil {
			for colNo, _ := range rows[0] {
				cols = append(cols, colNo+1)
			}
		} else if len(sheetObj.Columns) == 0 {
			for colNo, _ := range rows[0] {
				cols = append(cols, colNo+1)
			}
		} else {
			cols = sheetObj.Columns
		}
		type colValue interface{}

		for rowNo, row := range rows {
			var errs []string
			if rowNo+1 >= sheetObj.DataStartRow {
				sheetRow := make(map[string]interface{})
				for _, colNo := range cols {
					if len(row) > colNo-1 {
						var rowValue interface{}
						rowValue, err = strconv.ParseFloat(row[colNo-1], 64)
						if err != nil {
							rowValue, err = strconv.ParseBool(row[colNo-1])
							if err != nil {
								rowValue = row[colNo-1]
							}
						}
						sheetRow[colHeaders[colNo-1]] = rowValue
						field := schema.GetField(ctx, colHeaders[colNo-1])
						if field != nil {
							vErr := field.Validate(ctx, rowValue)
							if vErr != nil {
								errs = append(errs, vErr.Error())
							}
						}
					} else {
						sheetRow[colHeaders[colNo-1]] = ""
					}
				}
				if len(errs) > 0 {
					sheetRow["error"] = strings.Join(errs, " , ")
				}
				sheetData = append(sheetData, sheetRow)
			}
		}
		if readOutput == nil {
			readOutput = make(map[string]interface{})
		}
		readOutput[sheetName] = sheetData
	}

	return readOutput, nil
}
