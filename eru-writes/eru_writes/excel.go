package eru_writes

import (
	"errors"
	"fmt"
	"github.com/xuri/excelize/v2"
	"log"
	"math"
	"reflect"
	"time"
)

const (
	DataTypeString        = "string"
	DataTypeInt           = "int"
	DataTypeDate          = "date"
	DataTypeFloat         = "float"
	DataTypeBoolean       = "boolean"
	DefaultFloatPrecision = 2
	DefaultFloatByteSize  = 64
)

type ExcelWriteData struct {
	WriteData
	CellFormat CellFormatter
}

type CellFormatter struct {
	DataTypes []string
}

func (ewd *ExcelWriteData) WriteColumnar() (writeOutput []byte, err error) {
	if ewd.ColumnarDataMap == nil {
		if ewd.ColumnarData == nil || len(ewd.ColumnarData) == 0 {
			return nil, errors.New("excel data not found")
		} else {
			ewd.ColumnarDataMap = make(map[string][][]interface{})
			ewd.ColumnarDataMap["Sheet1"] = ewd.ColumnarData
		}
	}

	//log.Print(ewd.ColumnarDataMap)

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}()
	sheet1Found := false
	for k, v := range ewd.ColumnarDataMap {
		// Create a new sheet.
		log.Print("k = ", k)
		if k != "Sheet1" {
			sheetIdx, sheetErr := f.NewSheet(k)
			if err != nil {
				err = sheetErr
				log.Println(err)
				return
			}
			f.SetActiveSheet(sheetIdx)
		} else {
			sheet1Found = true
		}

		if len(ewd.ColumnarDataHeader) > 0 {
			for i, h := range ewd.ColumnarDataHeader {
				f.SetCellValue(k, fmt.Sprint(columnToLetter(i+1), 1), h)
			}
		}
		for rNo, row := range v {
			if !ewd.ColumnarDataHeaderFirstRow && len(ewd.ColumnarDataHeader) > 0 {
				rNo++
			}
			if !(rNo == 0 && ewd.ColumnarDataHeaderFirstRow && len(ewd.ColumnarDataHeader) > 0) {
				for cNo, col := range row {
					dt := DataTypeString
					if cNo < len(ewd.CellFormat.DataTypes) {
						dt = ewd.CellFormat.DataTypes[cNo]
					} else {
						dt = reflect.TypeOf(col).String()
					}
					switch dt {
					case DataTypeString:
						dtErr := f.SetCellStr(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
						if dtErr != nil {
							f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
						}
					case DataTypeInt:
						if _, valOk := col.(int); valOk {
							dtErr := f.SetCellInt(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(int))
							if dtErr != nil {
								f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
							}
						} else {
							f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
						}
					case DataTypeBoolean:
						if _, valOk := col.(bool); valOk {
							dtErr := f.SetCellBool(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(bool))
							if dtErr != nil {
								f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
							}
						} else {
							f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
						}
					case DataTypeFloat:
						if _, valOk := col.(float64); valOk {
							dtErr := f.SetCellFloat(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(float64), DefaultFloatPrecision, DefaultFloatByteSize)
							if dtErr != nil {
								f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
							}
						} else {
							f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
						}

					case DataTypeDate:
						log.Println(col.(string))
						dtVal, dtValErr := time.Parse("2006/01/02", col.(string))
						if dtValErr != nil {
							log.Println(dtValErr)
							f.SetCellValue(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
						} else {
							log.Println(dtVal)
							f.SetCellValue(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), dtVal)
						}
					default:
						if _, valOk := col.(string); valOk {
							f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), col.(string))
						} else {
							f.SetCellDefault(k, fmt.Sprint(columnToLetter(cNo+1), rNo+1), "Error!")
						}

					}
				}
			}
		}

		// Save spreadsheet by the given path.
		if ewd.FileName != "" {
			log.Print("saving file at ", ewd.FileName)
			if saveErr := f.SaveAs(fmt.Sprint(ewd.FileName)); saveErr != nil {
				err = saveErr
				log.Println(err)
			}
		}
	}
	if !sheet1Found {
		f.DeleteSheet("Sheet1")
	}
	b, bErr := f.WriteToBuffer()
	if bErr != nil {
		log.Println(bErr)
		return nil, bErr
	}
	return b.Bytes(), nil
}

func columnToLetter(column int) (letter string) {
	temp := 0

	for column > 0 {
		temp = (column - 1) % 26
		character := fmt.Sprintf("%c", temp+65)
		letter = fmt.Sprint(character, letter)
		column = (column - temp - 1) / 26
	}
	return letter
}

func letterToColumn(letter string) (column int) {
	length := len(letter)
	for i := 0; i < length; i++ {
		column += int(letter[i]-64) * int(math.Pow(26, float64(length-i-1)))
	}
	return column
}
