package eru_writes

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/xuri/excelize/v2"
)

const (
	DataTypeSmallInteger     = "SmallInteger"
	DataTypeInteger          = "Integer"
	DataTypeBigInteger       = "BigInteger"
	DataTypeDecimal          = "Decimal"
	DataTypeFloat            = "Float"
	DataTypeVarchar          = "Varchar"
	DataTypeChar             = "Char"
	DataTypeString           = "String"
	DataTypeDateTime         = "DateTime"
	DataTypeDateTimeWithZone = "DateTimeWithZone"
	DataTypeDate             = "Date"
	DataTypeTime             = "Time"
	DataTypeTimeWithZone     = "TimeWithZone"
	DataTypeBoolean          = "Boolean"
	DataTypeJSON             = "JSON"
	DefaultFloatPrecision    = 2
	DefaultFloatByteSize     = 64
	DefaultMaxColumnWidth    = 20.0
)

type ExcelWriteData struct {
	WriteData
	CellFormat  map[string]CellFormatter
	PivotConfig map[string]PivotTableConfig
}

func DefaultCellFormatter() *CellFormatter {
	return &CellFormatter{
		HeaderStyle: getDefaultHeaderStyle(),
		DataStyle:   getDefaultDataStyle(),
	}
}

type CellFormatter struct {
	HeaderStyle CellStyle `json:"header_style,omitempty"`
	DataStyle   CellStyle `json:"data_style,omitempty"`
}

type CellStyle struct {
	Font      *FontStyle      `json:"font,omitempty"`
	Fill      *FillStyle      `json:"fill,omitempty"`
	Border    []*BorderStyle  `json:"border,omitempty"`
	Alignment *AlignmentStyle `json:"alignment,omitempty"`
}

type FontStyle struct {
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
	Family string `json:"family,omitempty"`
	Size   int    `json:"size,omitempty"`
	Color  string `json:"color,omitempty"`
}

type FillStyle struct {
	Type    string   `json:"type,omitempty"`
	Color   []string `json:"color,omitempty"`
	Pattern int      `json:"pattern,omitempty"`
}

type BorderStyle struct {
	Type  string `json:"type,omitempty"`
	Color string `json:"color,omitempty"`
	Style int    `json:"style,omitempty"`
}

type AlignmentStyle struct {
	Horizontal string `json:"horizontal,omitempty"`
	Vertical   string `json:"vertical,omitempty"`
}

type PivotTableConfig struct {
	SheetName         string                   `json:"sheet_name"`
	DataRange         string                   `json:"-"`
	PivotRange        string                   `json:"-"`
	Rows              []string                 `json:"rows"`
	Columns           []string                 `json:"columns"`
	Aggregations      []PivotTableAggregations `json:"aggregations"`
	ShowColumnsTotals bool                     `json:"show_columns_totals"`
	ShowRowsTotals    bool                     `json:"show_rows_totals"`
}

type PivotTableAggregations struct {
	AggregationFunction string `json:"aggregation_function"`
	FieldName           string `json:"field_name"`
}

/* // Helper functions for safe access and defaults
func getDataType(dataTypes []string, index int) string {
	if index < len(dataTypes) && dataTypes[index] != "" {
		return dataTypes[index]
	}
	return DataTypeString
}

func getMaxWidth(maxWidths []float64, index int) float64 {
	if index < len(maxWidths) && maxWidths[index] > 0 {
		return maxWidths[index]
	}
	return DefaultMaxColumnWidth
}

func getCellStyle(styles []CellStyle, index int) *CellStyle {
	if index < len(styles) {
		return &styles[index]
	}
	return nil
} */

func getDefaultHeaderStyle() CellStyle {
	return CellStyle{
		Font: &FontStyle{
			Bold:   true,
			Family: "Arial",
			Size:   11,
			Color:  "#000000",
		},
		Alignment: &AlignmentStyle{
			Horizontal: "center",
			Vertical:   "center",
		},
	}
}
func getDefaultDataStyle() CellStyle {
	return CellStyle{
		Font: &FontStyle{
			Bold:   false,
			Family: "Arial",
			Size:   11,
			Color:  "#000000",
		},
		Alignment: &AlignmentStyle{
			Horizontal: getDefaultHorizontalAlignment(DataTypeString),
			Vertical:   "center",
		},
	}
}

func getDefaultHorizontalAlignment(dataType string) string {
	switch dataType {
	case DataTypeString:
		return "left"
	case DataTypeInteger, DataTypeBigInteger, DataTypeSmallInteger:
		return "right"
	case DataTypeFloat, DataTypeDecimal:
		return "right"
	case DataTypeDate, DataTypeDateTime, DataTypeDateTimeWithZone, DataTypeTime, DataTypeTimeWithZone:
		return "center"
	case DataTypeBoolean:
		return "center"
	case DataTypeJSON:
		return "left"
	default:
		return "left"
	}
}

// calculateOptimalColumnWidth calculates the optimal width for a column based on its content
func calculateOptimalColumnWidth(data [][]interface{}, columnIndex int) float64 {
	if len(data) == 0 || columnIndex >= len(data[0]) {
		return DefaultMaxColumnWidth
	}

	maxWidth := 0.0
	startRow := 0

	// Calculate width based on content
	for rowIndex := startRow; rowIndex < len(data); rowIndex++ {
		if columnIndex < len(data[rowIndex]) {
			cellValue := data[rowIndex][columnIndex]
			if cellValue != nil {
				cellStr := fmt.Sprintf("%v", cellValue)
				// Estimate width: roughly 1 character = 1 unit, with some padding
				estimatedWidth := float64(len(cellStr)) * 1.1
				if estimatedWidth > maxWidth {
					maxWidth = estimatedWidth
				}
			}
		}
	}

	// Add some padding for better readability
	maxWidth += 2.0

	// Ensure minimum width
	if maxWidth < 8.0 {
		maxWidth = 8.0
	}

	return maxWidth
}

// Helper function to calculate data range from sheet data
func calculateDataRange(sheetName string, data [][]interface{}, hasHeader bool) string {
	if len(data) == 0 {
		return fmt.Sprintf("%s!A1:A1", sheetName)
	}

	// Calculate dimensions
	rows := len(data)
	cols := 0
	if rows > 0 {
		cols = len(data[0])
	}

	// Convert to Excel range format
	endCol := columnToLetter(cols)
	endRow := rows

	return fmt.Sprintf("%s!A%d:%s%d", sheetName, 1, endCol, endRow)
}

// Helper function to calculate pivot table range
func calculatePivotRange(sheetName string, pivotConfig PivotTableConfig) string {
	// Calculate estimated pivot table size based on configuration
	estimatedRows := 10 // Base rows for headers and some data
	estimatedCols := 5  // Base columns

	// Add more rows/cols based on pivot configuration
	if len(pivotConfig.Rows) > 0 {
		estimatedRows += len(pivotConfig.Rows) * 3 // Estimate 3 rows per row field
	}
	if len(pivotConfig.Columns) > 0 {
		estimatedCols += len(pivotConfig.Columns) * 2 // Estimate 2 cols per column field
	}
	if len(pivotConfig.Aggregations) > 0 {
		estimatedCols += len(pivotConfig.Aggregations) // Add columns for measures
	}

	// Ensure minimum size
	if estimatedRows < 5 {
		estimatedRows = 5
	}
	if estimatedCols < 3 {
		estimatedCols = 3
	}

	endCol := columnToLetter(estimatedCols)
	return fmt.Sprintf("%s!A1:%s%d", sheetName, endCol, estimatedRows)
}

// parseCellReference parses a cell reference like "A1" into row and column numbers
func parseCellReference(cellRef string) (int, int) {
	colStr := ""
	rowStr := ""

	for i, char := range cellRef {
		if char >= 'A' && char <= 'Z' {
			colStr += string(char)
		} else if char >= '0' && char <= '9' {
			rowStr = cellRef[i:]
			break
		}
	}

	row, _ := strconv.Atoi(rowStr)
	col := letterToColumn(colStr)

	return row, col
}

// autoFitPivotTableColumns auto-fits columns in the pivot table range
func autoFitPivotTableColumns(f *excelize.File, sheetName, pivotRange string) error {
	// Parse the range to get start and end columns
	parts := strings.Split(pivotRange, "!")
	if len(parts) != 2 {
		return fmt.Errorf("invalid pivot range format: %s", pivotRange)
	}

	rangePart := parts[1]
	rangeParts := strings.Split(rangePart, ":")
	if len(rangeParts) != 2 {
		return fmt.Errorf("invalid range format: %s", rangePart)
	}

	startCell := rangeParts[0]
	endCell := rangeParts[1]

	// Extract column letters
	startCol := ""
	endCol := ""

	for _, char := range startCell {
		if char >= 'A' && char <= 'Z' {
			startCol += string(char)
		} else {
			break
		}
	}

	for _, char := range endCell {
		if char >= 'A' && char <= 'Z' {
			endCol += string(char)
		} else {
			break
		}
	}

	// Convert column letters to numbers for iteration
	startColNum := letterToColumn(startCol)
	endColNum := letterToColumn(endCol)

	// Auto-fit each column in the range
	for colNum := startColNum; colNum <= endColNum; colNum++ {
		colLetter := columnToLetter(colNum)
		// Calculate optimal width based on content
		optimalWidth := calculateOptimalColumnWidthForPivot(f, sheetName, colNum)
		if err := f.SetColWidth(sheetName, colLetter, colLetter, optimalWidth); err != nil {
			// Log warning but continue with other columns
			fmt.Printf("Warning: Failed to set column width for %s: %v\n", colLetter, err)
		}
	}

	return nil
}

// calculateOptimalColumnWidthForPivot calculates optimal width for pivot table columns
func calculateOptimalColumnWidthForPivot(f *excelize.File, sheetName string, colNum int) float64 {
	// Get all rows in the sheet to calculate optimal width
	rows, err := f.GetRows(sheetName)
	if err != nil || len(rows) == 0 {
		return DefaultMaxColumnWidth
	}

	maxWidth := 0.0

	// Check each row for this column
	for _, row := range rows {
		if colNum-1 < len(row) && row[colNum-1] != "" {
			cellStr := row[colNum-1]
			// Estimate width: roughly 1 character = 1 unit, with some padding
			estimatedWidth := float64(len(cellStr)) * 1.1
			if estimatedWidth > maxWidth {
				maxWidth = estimatedWidth
			}
		}
	}

	// Add some padding for better readability
	maxWidth += 2.0

	// Ensure minimum width
	if maxWidth < 8.0 {
		maxWidth = 8.0
	}

	// Cap at maximum width
	if maxWidth > DefaultMaxColumnWidth {
		maxWidth = DefaultMaxColumnWidth
	}

	return maxWidth
}

// Helper functions for safe type conversion
func safeString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

func safeInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}

func safeFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func safeBool(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		lower := strings.ToLower(v)
		if lower == "true" || lower == "1" || lower == "yes" {
			return true, true
		}
		if lower == "false" || lower == "0" || lower == "no" {
			return false, true
		}
	}
	return false, false
}

func safeDate(value interface{}) (time.Time, bool) {
	if str, ok := value.(string); ok {
		// Try multiple date formats
		formats := []string{
			"2006-01-02",
			"2006/01/02",
			"01/02/2006",
			"02/01/2006",
			"2006-01-02 15:04:05",
			"2006/01/02 15:04:05",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, str); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// Helper function to create style from CellStyle
func createStyle(f *excelize.File, style CellStyle) (int, error) {
	if f == nil {
		return 0, errors.New("excelize file is nil")
	}

	excelStyle := &excelize.Style{}

	if style.Font != nil {
		excelStyle.Font = &excelize.Font{}
		if style.Font.Bold {
			excelStyle.Font.Bold = true
		}
		if style.Font.Italic {
			excelStyle.Font.Italic = true
		}
		if style.Font.Family != "" {
			excelStyle.Font.Family = style.Font.Family
		}
		if style.Font.Size > 0 {
			excelStyle.Font.Size = float64(style.Font.Size)
		}
		if style.Font.Color != "" {
			excelStyle.Font.Color = style.Font.Color
		}
	}

	if style.Fill != nil {
		excelStyle.Fill = excelize.Fill{}
		if style.Fill.Type != "" {
			excelStyle.Fill.Type = style.Fill.Type
		}
		if len(style.Fill.Color) > 0 {
			excelStyle.Fill.Color = style.Fill.Color
		}
		if style.Fill.Pattern > 0 {
			excelStyle.Fill.Pattern = style.Fill.Pattern
		}
	}

	if style.Border != nil {
		excelStyle.Border = []excelize.Border{}
		for _, border := range style.Border {
			if border.Type == "" {
				excelStyle.Border = append(excelStyle.Border, excelize.Border{
					Type:  "left",
					Color: border.Color,
					Style: border.Style,
				})
				excelStyle.Border = append(excelStyle.Border, excelize.Border{
					Type:  "right",
					Color: border.Color,
					Style: border.Style,
				})
				excelStyle.Border = append(excelStyle.Border, excelize.Border{
					Type:  "top",
					Color: border.Color,
					Style: border.Style,
				})
				excelStyle.Border = append(excelStyle.Border, excelize.Border{
					Type:  "bottom",
					Color: border.Color,
					Style: border.Style,
				})
			} else {
				excelStyle.Border = append(excelStyle.Border, excelize.Border{
					Type:  border.Type,
					Color: border.Color,
					Style: border.Style,
				})
			}
		}
	}

	if style.Alignment != nil {
		excelStyle.Alignment = &excelize.Alignment{}
		if style.Alignment.Horizontal != "" {
			excelStyle.Alignment.Horizontal = style.Alignment.Horizontal
		}
		if style.Alignment.Vertical != "" {
			excelStyle.Alignment.Vertical = style.Alignment.Vertical
		}
	}

	// Only create style if there's something to style
	hasFill := excelStyle.Fill.Type != "" || len(excelStyle.Fill.Color) > 0 || excelStyle.Fill.Pattern > 0
	hasBorder := len(excelStyle.Border) > 0
	if excelStyle.Font == nil && !hasFill &&
		!hasBorder && excelStyle.Alignment == nil {
		return 0, nil // No style to apply
	}

	return f.NewStyle(excelStyle)
}

func (ewd *ExcelWriteData) WriteColumnar(ctx context.Context) (writeOutput []byte, err error) {
	logs.WithContext(ctx).Debug("WriteColumnar - Start")

	if ewd.ColumnarDataMap == nil {
		return nil, errors.New("excel data not found")
	}
	/* if ewd.ColumnarDataMap == nil {
		if len(ewd.ColumnarData) == 0 {
			return nil, errors.New("excel data not found")
		} else {
			ewd.ColumnarDataMap = make(map[string][][]interface{})
			ewd.ColumnarDataMap["Sheet1"] = ewd.ColumnarData
		}
	} */

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
	}()
	sheet1Found := false
	for k, v := range ewd.ColumnarDataMap {
		sheetSettings := ColumnarSettings{
			HeaderFirstRow: true,
			Headers:        make(map[int]ColumnHeaders),
		}
		if ewd.ColumnarSettings != nil {
			if settings, exists := ewd.ColumnarSettings[k]; exists {
				sheetSettings = settings
			}
		}

		if ewd.CellFormat == nil {
			ewd.CellFormat = make(map[string]CellFormatter)
		}
		if cellFormatter, cellFormatterOk := ewd.CellFormat[k]; cellFormatterOk {
			ewd.CellFormat[k] = *mergeCellFormatters(&cellFormatter, DefaultCellFormatter())
		} else {
			ewd.CellFormat[k] = *DefaultCellFormatter()
		}

		// Create a new sheet.
		if k != "Sheet1" {
			sheetIdx, sheetErr := f.NewSheet(k)
			if sheetErr != nil {
				err = sheetErr
				logs.WithContext(ctx).Error(err.Error())
				return
			}
			f.SetActiveSheet(sheetIdx)
		} else {
			sheet1Found = true
		}

		/* // Set headers with styling
		if len(ewd.ColumnarDataHeader) > 0 {
			for i, h := range ewd.ColumnarDataHeader {
				cellFormatter, cellFormatterOk := sheelCellFormat[h]
				if !cellFormatterOk {
					cellFormatter = DefaultCellFormatter()
				} else {
					cellFormatter = *mergeCellFormatters(&cellFormatter)
				}
				cellRef := fmt.Sprint(columnToLetter(i+1), 1)
				f.SetCellValue(k, cellRef, h)

				// Apply header style - use provided style or default
				if styleID, styleErr := createStyle(f, cellFormatter.HeaderStyle); styleErr == nil {
					f.SetCellStyle(k, cellRef, cellRef, styleID)
				}
			}
		} */

		// Process data rows
		for rNo, row := range v {
			for cNo, col := range row {
				cellRef := fmt.Sprint(columnToLetter(cNo+1), rNo+1)

				// Determine data type - use provided type or auto-detect
				dt := sheetSettings.Headers[cNo].DataType
				if dt == DataTypeString {
					// Auto-detect type if not specified
					dt = reflect.TypeOf(col).String()
				}

				// Set cell value with proper data type
				var setErr error
				switch dt {
				case DataTypeString:
					setErr = f.SetCellStr(k, cellRef, safeString(col))
				case DataTypeInteger, DataTypeBigInteger, DataTypeSmallInteger:
					if intVal, ok := safeInt(col); ok {
						setErr = f.SetCellInt(k, cellRef, int64(intVal))
					} else {
						setErr = f.SetCellStr(k, cellRef, safeString(col))
					}
				case DataTypeBoolean:
					if boolVal, ok := safeBool(col); ok {
						setErr = f.SetCellBool(k, cellRef, boolVal)
					} else {
						setErr = f.SetCellStr(k, cellRef, safeString(col))
					}
				case DataTypeFloat, DataTypeDecimal:
					if floatVal, ok := safeFloat(col); ok {
						setErr = f.SetCellFloat(k, cellRef, floatVal, DefaultFloatPrecision, DefaultFloatByteSize)
					} else {
						setErr = f.SetCellStr(k, cellRef, safeString(col))
					}
				case DataTypeDate, DataTypeDateTime, DataTypeDateTimeWithZone, DataTypeTime, DataTypeTimeWithZone:
					if dateVal, ok := safeDate(col); ok {
						setErr = f.SetCellValue(k, cellRef, dateVal)
					} else {
						setErr = f.SetCellStr(k, cellRef, safeString(col))
					}
				default:
					setErr = f.SetCellStr(k, cellRef, safeString(col))
				}

				if setErr != nil {
					logs.WithContext(ctx).Warn(fmt.Sprintf("Failed to set cell %s: %v", cellRef, setErr))
					f.SetCellStr(k, cellRef, safeString(col))
				}

			}
			//	}
		}

		startDataCellRef := fmt.Sprint(columnToLetter(1), 1)
		endDataCellRef := fmt.Sprint(columnToLetter(len(v[0])), len(v))
		if sheetSettings.HeaderFirstRow {
			startHeaderCellRef := fmt.Sprint(columnToLetter(1), 1)
			endHeaderCellRef := fmt.Sprint(columnToLetter(len(v[0])), 1)
			startDataCellRef = fmt.Sprint(columnToLetter(1), 2)
			if styleID, styleErr := createStyle(f, ewd.CellFormat[k].HeaderStyle); styleErr == nil {
				f.SetCellStyle(k, startHeaderCellRef, endHeaderCellRef, styleID)
			}
		}
		if styleID, styleErr := createStyle(f, ewd.CellFormat[k].DataStyle); styleErr == nil {
			f.SetCellStyle(k, startDataCellRef, endDataCellRef, styleID)
		}

		//Add pivot here if confg is provided
		if ewd.PivotConfig != nil {
			pivotConfig, pivotConfigOk := ewd.PivotConfig[k]
			if pivotConfigOk {
				pivotConfig.DataRange = calculateDataRange(k, v, sheetSettings.HeaderFirstRow)

				if pivotConfig.SheetName == "" {
					pivotConfig.SheetName = fmt.Sprintf("%s_pivot", k)
				}

				// Add the pivot table sheet
				sheetIdx, err := f.NewSheet(pivotConfig.SheetName)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, err
				}
				f.SetActiveSheet(sheetIdx)

				// Prepare pivot table options
				pivotOptions := excelize.PivotTableOptions{
					UseAutoFormatting: true,
					ClassicLayout:     true,
					ShowColHeaders:    true,
					DataRange:         pivotConfig.DataRange,
					RowGrandTotals:    pivotConfig.ShowRowsTotals,
					ColGrandTotals:    pivotConfig.ShowColumnsTotals,
					PivotTableRange:   fmt.Sprintf("%s!A1:B2", pivotConfig.SheetName),
					Rows:              make([]excelize.PivotTableField, len(pivotConfig.Rows)),
					Columns:           make([]excelize.PivotTableField, len(pivotConfig.Columns)),
					Data:              make([]excelize.PivotTableField, len(pivotConfig.Aggregations)),
				}

				// Set row fields with subtotal information
				for i, row := range pivotConfig.Rows {
					pivotOptions.Rows[i] = excelize.PivotTableField{
						Data:            row,
						Name:            row,
						DefaultSubtotal: ewd.getFieldSubTotal(k, i),
					}
				}

				// Set column fields with subtotal information
				for i, col := range pivotConfig.Columns {
					pivotOptions.Columns[i] = excelize.PivotTableField{
						Data:            col,
						Name:            col,
						DefaultSubtotal: ewd.getFieldSubTotal(k, i),
					}
				}

				// Set data fields (measures)
				for i, measure := range pivotConfig.Aggregations {
					pivotOptions.Data[i] = excelize.PivotTableField{
						Data:     measure.FieldName,
						Name:     measure.FieldName,
						Subtotal: measure.AggregationFunction,
					}
				}

				// Add pivot table
				if err := f.AddPivotTable(&pivotOptions); err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, err
				}

				// Use the actual pivot table range for styling
				actualPivotRange := calculatePivotRange(pivotConfig.SheetName, pivotConfig)

				if err := autoFitPivotTableColumns(f, pivotConfig.SheetName, actualPivotRange); err != nil {
					logs.WithContext(ctx).Warn(fmt.Sprintf("Failed to auto-fit pivot table columns: %v", err))
				}
			}
		}

		// Set column widths with auto-fit and max width constraints
		if len(v) > 0 {
			for cNo := 0; cNo < len(v[0]); cNo++ {
				maxWidthConstraint := sheetSettings.Headers[cNo].MaxWidth
				if maxWidthConstraint == 0 {
					maxWidthConstraint = DefaultMaxColumnWidth
				}
				colLetter := columnToLetter(cNo + 1)

				// Calculate optimal width based on content (auto-fit)
				optimalWidth := calculateOptimalColumnWidth(v, cNo)

				// Use the smaller of optimal width or max constraint
				finalWidth := optimalWidth
				if optimalWidth > maxWidthConstraint {
					finalWidth = maxWidthConstraint
				}
				f.SetColWidth(k, colLetter, colLetter, finalWidth)
			}
		}

		// Save spreadsheet by the given path.
		if ewd.FileName != "" {
			logs.WithContext(ctx).Info(fmt.Sprint("saving file at ", ewd.FileName))
			if saveErr := f.SaveAs(fmt.Sprint(ewd.FileName)); saveErr != nil {
				err = saveErr
				logs.WithContext(ctx).Error(err.Error())
			}
		}
	}
	if !sheet1Found {
		f.DeleteSheet("Sheet1")
	}
	b, bErr := f.WriteToBuffer()
	if bErr != nil {
		logs.WithContext(ctx).Error(bErr.Error())
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

// getFieldSubTotal returns the SubTotal setting for a field from CellFormatter
func (ewd *ExcelWriteData) getFieldSubTotal(sheetName string, cNo int) bool {
	if ewd.ColumnarSettings == nil {
		return false
	}

	sheetSettings, exists := ewd.ColumnarSettings[sheetName]
	if !exists {
		return false
	}

	if subTotalHeader, exists := sheetSettings.Headers[cNo]; exists {
		if subTotalHeader.SubTotal {
			return true
		}
	}
	return false
}

func mergeCellFormatters(cellFormatter *CellFormatter, defaultFormatter *CellFormatter) *CellFormatter {
	if cellFormatter.HeaderStyle.Font == nil {
		cellFormatter.HeaderStyle.Font = defaultFormatter.HeaderStyle.Font
	}
	if cellFormatter.HeaderStyle.Fill == nil {
		cellFormatter.HeaderStyle.Fill = defaultFormatter.HeaderStyle.Fill
	}
	if cellFormatter.HeaderStyle.Border == nil {
		cellFormatter.HeaderStyle.Border = defaultFormatter.HeaderStyle.Border
	}
	if cellFormatter.HeaderStyle.Alignment == nil {
		cellFormatter.HeaderStyle.Alignment = defaultFormatter.HeaderStyle.Alignment
	}
	if cellFormatter.DataStyle.Font == nil {
		cellFormatter.DataStyle.Font = defaultFormatter.DataStyle.Font
	}
	if cellFormatter.DataStyle.Fill == nil {
		cellFormatter.DataStyle.Fill = defaultFormatter.DataStyle.Fill
	}
	if cellFormatter.DataStyle.Border == nil {
		cellFormatter.DataStyle.Border = defaultFormatter.DataStyle.Border
	}
	if cellFormatter.DataStyle.Alignment == nil {
		cellFormatter.DataStyle.Alignment = defaultFormatter.DataStyle.Alignment
	}
	return cellFormatter
}

// mergeCellFormattersMap merges two CellFormatter maps, prioritizing keys from the first parameter
func MergeCellFormattersMap(primary, secondary map[string]CellFormatter) map[string]CellFormatter {
	result := make(map[string]CellFormatter)

	kDone := []string{}
	for k, v := range primary {
		if sv, exists := secondary[k]; exists {
			result[k] = *mergeCellFormatters(&v, &sv)
		} else {
			result[k] = v
		}
		kDone = append(kDone, k)
	}
	for k, v := range secondary {
		if !slices.Contains(kDone, k) {
			result[k] = v
		}
	}
	return result
}

// mergePivotConfigs merges two PivotTableConfig maps, prioritizing keys from the first parameter
func MergePivotConfigs(primary, secondary map[string]PivotTableConfig) map[string]PivotTableConfig {
	result := make(map[string]PivotTableConfig)

	// First, copy all values from secondary (fallback)
	for k, v := range secondary {
		result[k] = v
	}

	// Then, override with values from primary (priority)
	for k, v := range primary {
		result[k] = v
	}

	return result
}

// mergeColumnarSettings merges two ColumnarSettings maps, prioritizing keys from the first parameter
func MergeColumnarSettings(primary, secondary map[string]ColumnarSettings) map[string]ColumnarSettings {
	result := make(map[string]ColumnarSettings)

	// First, copy all values from secondary (fallback)
	for k, v := range secondary {
		result[k] = v
	}

	// Then, override with values from primary (priority)
	for k, v := range primary {
		result[k] = v
	}

	return result
}
