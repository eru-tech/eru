package eru_writes

import (
	"context"
	"errors"
	"fmt"
)

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
	WriteColumnar(ctx context.Context) (writeOutput []byte, err error)
	MapToCsv(ctx context.Context, mapObjs []map[string]interface{}, firstRowHeader bool) (writeOutput []byte, err error)
}

func (wd *WriteData) WriteColumnar() (writeOutput []byte, err error) {
	return nil, errors.New("WriteColumnar method not supported")
}

func extractHeaders(records []interface{}) (headers []string, validRecords []map[string]interface{}) {
	headerSet := make(map[string]struct{})
	for _, record := range records {
		if rec, recOk := record.(map[string]interface{}); recOk {
			validRecords = append(validRecords, rec)
			for key := range rec {
				headerSet[key] = struct{}{}
			}
		}
	}
	// Convert the set to a slice
	for key := range headerSet {
		headers = append(headers, key)
	}
	return
}

// buildRow constructs a row for the given record based on the headers
func buildRow(record map[string]interface{}, headers []string) []string {
	row := make([]string, len(headers))
	for i, header := range headers {
		if value, exists := record[header]; exists {
			row[i] = fmt.Sprintf("%v", value) // Convert any type to a string
		} else {
			row[i] = "" // Leave blank if key is not present
		}
	}
	return row
}
