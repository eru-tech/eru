package eru_writes

import (
	"log"
	"testing"
)

func TestColumnToLetter(t *testing.T) {
	col := 65
	log.Print(col, " = ", columnToLetter(col), " : ", letterToColumn(columnToLetter(col)))

	col = 0
	log.Print(col, " = ", columnToLetter(col), " : ", letterToColumn(columnToLetter(col)))

	col = 1
	log.Print(col, " = ", columnToLetter(col), " : ", letterToColumn(columnToLetter(col)))

	col = 321
	log.Print(col, " = ", columnToLetter(col), " : ", letterToColumn(columnToLetter(col)))

}
func TestWriteColumnar(t *testing.T) {
	ewd := ExcelWriteData{}
	ewd.FileName = "/home/alty/mycode/goworkspace/eru/eru_writes/eru_writes/abc.xlsx"
	ewd.ColumnarData = [][]interface{}{{"a", "b", "c"}, {1, "a", true, 4.589, "2023/08/30"}, {"2", "b", "false", 9.991, "2023/30/08"}}
	//ewd.ColumnarDataMap = make(map[string][][]interface{})
	//ewd.ColumnarDataMap["xxx"] = ewd.ColumnarData
	ewd.ColumnarDataHeader = []string{"A", "B", "C"}
	ewd.ColumnarDataHeaderFirstRow = false
	cf := CellFormatter{}
	cf.DataTypes = []string{"int", "string", "boolean", "float", "date"}
	ewd.CellFormat = cf
	ewd.WriteColumnar()
}
