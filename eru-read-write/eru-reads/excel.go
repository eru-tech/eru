package eru_reads

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-read-write/validator"
	"github.com/xuri/excelize/v2"
	"math/big"
	"strconv"
	"strings"
	"time"
)

var dateFormats = []string{
	"01/02/2006",      // Example: 12/31/1999
	"01-02-2006",      // Example: 12-31-1999
	"2006-01-02",      // Example: 1999-12-31
	"January 2, 2006", // Example: December 31, 1999
	"2 January 2006",  // Example: 31 December 1999
	"02-Jan-2006",     // Example: 31-Dec-1999
	"2006-01-02T15:04:05-07:00",
}

type ExcelReadData struct {
	ReadData
	Sheets map[string]FileReadData `json:"sheets"`
}

func (erd *ExcelReadData) ReadAsJson(ctx context.Context, readData []byte) (readOutput map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("WriteColumnar - Start")
	logs.WithContext(ctx).Info(fmt.Sprint(erd.Sheets))
	f, err := excelize.OpenReader(bytes.NewReader(readData), excelize.Options{
		RawCellValue: true,
	}, excelize.Options{
		LongDatePattern: "yyyy-mm-dd",
	}, excelize.Options{
		ShortDatePattern: "yyyy-mm-dd",
	})
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
		rows, rowsErr := f.GetRows(sheetName)
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
		//type colValue interface{}

		for rowNo, row := range rows {
			var errs []string
			if rowNo+1 >= sheetObj.DataStartRow {
				sheetRow := make(map[string]interface{})
				for _, colNo := range cols {

					isNum := false
					var rowValue interface{}
					var rowValueF float64
					field := schema.GetField(ctx, colHeaders[colNo-1])

					if len(row) > colNo-1 {
						if field.GetDatatype() == "date" {
							formatMatched := false
							for _, format := range dateFormats {
								if dateValue, err := time.Parse(format, row[colNo-1]); err == nil {
									logs.WithContext(ctx).Info(fmt.Sprint(dateValue))
									rowValue = dateValue.Format("2006-01-02")
									logs.WithContext(ctx).Info(fmt.Sprint(rowValue))
									formatMatched = true
									break
								} else {
									logs.WithContext(ctx).Error(err.Error())
								}
							}
							if !formatMatched {
								rowValue = row[colNo-1]
							}
						} else {
							rowValueF, err = strconv.ParseFloat(row[colNo-1], 64)
							if err != nil {
								rowValue, err = strconv.ParseBool(row[colNo-1])
								if err != nil {
									rowValue = row[colNo-1]
								}
							} else {
								rowValue = rowValueF
								isNum = true
							}
						}
					} else {
						rowValue = nil
					}
					sheetRow[colHeaders[colNo-1]] = rowValue

					if field != nil {
						logs.WithContext(ctx).Info(fmt.Sprint("field.GetDatatype() = ", field.GetDatatype(), " isNum = ", isNum, " rowValueF = ", rowValueF, " rowValue = ", rowValue))
						if field.GetDatatype() == "date" && isNum {
							var vTime time.Time
							vTime, err = excelize.ExcelDateToTime(rowValueF, false)
							if err != nil {
								logs.WithContext(ctx).Error(err.Error())
								errs = append(errs, err.Error())
							}
							sheetRow[colHeaders[colNo-1]] = vTime.Format("2006-01-02")
							rowValue = vTime.Format("2006-01-02")
						} else if field.GetDatatype() == "number" && !isNum && rowValue != nil {
							if rowValue.(string) == "" {
								rowValue = nil
								sheetRow[colHeaders[colNo-1]] = rowValue
							}
						} else if field.GetDatatype() == "array" {
							var rowValueArray []interface{}
							if rowValue == nil {
								rowValueArray = make([]interface{}, 0)
							} else if arrayStr, arrayStrOk := rowValue.(string); arrayStrOk {
								if arrayStr == "" {
									rowValueArray = make([]interface{}, 0)
								} else {
									arrayVal := strings.Split(arrayStr, ",")
									ary := make([]float64, len(arrayVal))
									for i, sa := range arrayVal {
										ary[i], err = strconv.ParseFloat(sa, 64)
										if err != nil {
											rowValueArray = append(rowValueArray, sa)
										} else {
											rowValueArray = append(rowValueArray, ary[i])
										}
									}
								}
							} else if isNum {
								rowValueArray = append(rowValueArray, rowValue)
							}
							sheetRow[colHeaders[colNo-1]] = rowValueArray
							rowValue = rowValueArray
						}
						vErr := field.Validate(ctx, rowValue)
						if vErr != nil {
							logs.WithContext(ctx).Error(vErr.Error())
							errs = append(errs, vErr.Error())
						}
						if field.ToEncode(ctx) {
							rowBytes := []byte("")
							rowBytes, err = json.Marshal(rowValue)
							if err != nil {
								errs = append(errs, err.Error())
							}
							rowStr := ""
							rowStr, err = strconv.Unquote(string(rowBytes))
							if err != nil {
								errs = append(errs, err.Error())
							}
							sheetRow[colHeaders[colNo-1]] = b64.StdEncoding.EncodeToString([]byte(rowStr))
						} else if field.GetDatatype() == "string" && isNum {
							bigint := big.NewFloat(rowValueF)
							rowValue = bigint.String()
							sheetRow[colHeaders[colNo-1]] = rowValue
						}
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
