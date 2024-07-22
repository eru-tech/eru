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
		//type colValue interface{}

		for rowNo, row := range rows {
			var errs []string
			if rowNo+1 >= sheetObj.DataStartRow {
				sheetRow := make(map[string]interface{})
				for _, colNo := range cols {
					colName, err := excelize.ColumnNumberToName(colNo)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						errs = append(errs, err.Error())
					}
					cell := colName + strconv.Itoa(rowNo+1)
					isNum := false
					var rowValue interface{}
					var rowValueF float64
					field := schema.GetField(ctx, colHeaders[colNo-1])
					styleID, styleErr := f.GetCellStyle(sheetName, cell)
					if styleErr != nil {
						logs.WithContext(ctx).Error(styleErr.Error())
						errs = append(errs, styleErr.Error())
					}
					// Get the cell style details
					style, err := f.GetStyle(styleID)
					if err != nil {
						logs.WithContext(ctx).Error(styleErr.Error())
						errs = append(errs, styleErr.Error())
					}
					logs.WithContext(ctx).Info(fmt.Sprint("style of cell", cell, " is ", style.NumFmt))
					// Get the cell type
					_, err = f.GetCellType(sheetName, cell)
					if err != nil {
						logs.WithContext(ctx).Error(styleErr.Error())
						errs = append(errs, styleErr.Error())
					}
					if len(row) > colNo-1 {
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
					} else {
						rowValue = nil
					}
					sheetRow[colHeaders[colNo-1]] = rowValue

					if field != nil {
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
	logs.WithContext(ctx).Info(fmt.Sprint(readOutput))
	return readOutput, nil
}
