package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type xlsxSheet struct {
	Name string
	Rows [][]string
}

type xlsxWorksheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Ref       string        `xml:"r,attr"`
	Type      string        `xml:"t,attr"`
	Value     string        `xml:"v"`
	InlineStr xlsxInlineStr `xml:"is"`
}

type xlsxInlineStr struct {
	Text string `xml:"t"`
}

func readXLSXRows(reader io.ReaderAt, size int64) ([][]string, error) {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	for _, file := range zipReader.File {
		files[file.Name] = file
	}
	sharedStrings, err := readSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}
	sheetFile := firstWorksheet(files)
	if sheetFile == nil {
		return nil, fmt.Errorf("xlsx 未找到工作表")
	}
	content, err := readZipFile(sheetFile)
	if err != nil {
		return nil, err
	}
	var worksheet xlsxWorksheet
	if err := xml.Unmarshal(content, &worksheet); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(worksheet.Rows))
	for _, row := range worksheet.Rows {
		values := []string{}
		for i, cell := range row.Cells {
			col := xlsxColumnIndex(cell.Ref)
			if col < 0 {
				col = i
			}
			for len(values) <= col {
				values = append(values, "")
			}
			values[col] = xlsxCellText(cell, sharedStrings)
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func writeAssetXLSX(c *gin.Context, filename string, sheets []xlsxSheet) {
	content, err := buildXLSX(sheets)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成 Excel 模板失败")
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func buildXLSX(sheets []xlsxSheet) ([]byte, error) {
	if len(sheets) == 0 {
		sheets = []xlsxSheet{{Name: "Sheet1"}}
	}
	var body bytes.Buffer
	zipWriter := zip.NewWriter(&body)
	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypes(len(sheets)),
		"_rels/.rels":                xlsxRootRels(),
		"xl/workbook.xml":            xlsxWorkbook(sheets),
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRels(len(sheets)),
		"xl/styles.xml":              xlsxStyles(),
		"docProps/core.xml":          xlsxCoreProps(),
		"docProps/app.xml":           xlsxAppProps(),
	}
	for name, content := range files {
		if err := writeXLSXZipFile(zipWriter, name, content); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}
	for i, sheet := range sheets {
		if err := writeXLSXZipFile(zipWriter, fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), xlsxWorksheetXML(sheet.Rows)); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func writeXLSXZipFile(zipWriter *zip.Writer, name, content string) error {
	file, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte(content))
	return err
}

func xlsxContentTypes(sheetCount int) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	builder.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	builder.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	builder.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	builder.WriteString(`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for i := 1; i <= sheetCount; i++ {
		builder.WriteString(fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i))
	}
	builder.WriteString(`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	builder.WriteString(`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`)
	builder.WriteString(`</Types>`)
	return builder.String()
}

func xlsxRootRels() string {
	return `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/></Relationships>`
}

func xlsxWorkbook(sheets []xlsxSheet) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	for i, sheet := range sheets {
		name := strings.TrimSpace(sheet.Name)
		if name == "" {
			name = fmt.Sprintf("Sheet%d", i+1)
		}
		builder.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, xlsxEscapeAttr(name), i+1, i+1))
	}
	builder.WriteString(`</sheets></workbook>`)
	return builder.String()
}

func xlsxWorkbookRels(sheetCount int) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 1; i <= sheetCount; i++ {
		builder.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i, i))
	}
	builder.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, sheetCount+1))
	builder.WriteString(`</Relationships>`)
	return builder.String()
}

func xlsxStyles() string {
	return `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF1F4E79"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border><border><left style="thin"><color rgb="FFD9E2EC"/></left><right style="thin"><color rgb="FFD9E2EC"/></right><top style="thin"><color rgb="FFD9E2EC"/></top><bottom style="thin"><color rgb="FFD9E2EC"/></bottom><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1"/><xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"/><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1" applyAlignment="1"><alignment wrapText="1" vertical="top"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`
}

func xlsxCoreProps() string {
	return `<?xml version="1.0" encoding="UTF-8"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>资产导入模板</dc:title><dc:creator>AssetRegistrationManagementSystem</dc:creator></cp:coreProperties>`
}

func xlsxAppProps() string {
	return `<?xml version="1.0" encoding="UTF-8"?><Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>AssetRegistrationManagementSystem</Application></Properties>`
}

func xlsxWorksheetXML(rows [][]string) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
	builder.WriteString(xlsxColumnsXML(rows))
	builder.WriteString(`<sheetData>`)
	for rowIndex, row := range rows {
		style := "2"
		height := ""
		if rowIndex == 0 {
			style = "1"
			height = ` ht="24" customHeight="1"`
		}
		builder.WriteString(fmt.Sprintf(`<row r="%d"%s>`, rowIndex+1, height))
		for colIndex, value := range row {
			cellRef := fmt.Sprintf("%s%d", xlsxColumnName(colIndex), rowIndex+1)
			builder.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr" s="%s"><is><t>%s</t></is></c>`, cellRef, style, xlsxEscapeText(value)))
		}
		builder.WriteString(`</row>`)
	}
	builder.WriteString(`</sheetData></worksheet>`)
	return builder.String()
}

func xlsxColumnsXML(rows [][]string) string {
	maxColumns := 1
	for _, row := range rows {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
	}
	var builder strings.Builder
	builder.WriteString(`<cols>`)
	for i := 0; i < maxColumns; i++ {
		width := 14
		if i == 0 {
			width = 18
		}
		if i >= 21 && i <= 24 {
			width = 16
		}
		builder.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, width))
	}
	builder.WriteString(`</cols>`)
	return builder.String()
}

func xlsxEscapeText(value string) string {
	var builder strings.Builder
	if err := xml.EscapeText(&builder, []byte(value)); err != nil {
		return ""
	}
	return builder.String()
}

func xlsxEscapeAttr(value string) string {
	return xlsxEscapeText(value)
}

func firstWorksheet(files map[string]*zip.File) *zip.File {
	names := make([]string, 0)
	for name := range files {
		if strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}
	return files[names[0]]
}

func readSharedStrings(file *zip.File) ([]string, error) {
	if file == nil {
		return nil, nil
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	decoder := xml.NewDecoder(rc)
	values := []string{}
	inString := false
	inText := false
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if typed.Name.Local == "si" {
				inString = true
				builder.Reset()
			}
			if inString && typed.Name.Local == "t" {
				inText = true
			}
		case xml.CharData:
			if inString && inText {
				builder.Write([]byte(typed))
			}
		case xml.EndElement:
			if typed.Name.Local == "t" {
				inText = false
			}
			if typed.Name.Local == "si" {
				values = append(values, builder.String())
				inString = false
			}
		}
	}
	return values, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func xlsxCellText(cell xlsxCell, sharedStrings []string) string {
	switch cell.Type {
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err == nil && index >= 0 && index < len(sharedStrings) {
			return sharedStrings[index]
		}
	case "inlineStr":
		return cell.InlineStr.Text
	}
	return strings.TrimSpace(cell.Value)
}

func xlsxColumnIndex(ref string) int {
	index := -1
	for _, ch := range ref {
		if ch < 'A' || ch > 'Z' {
			break
		}
		if index < 0 {
			index = 0
		}
		index = index*26 + int(ch-'A'+1)
	}
	if index < 0 {
		return -1
	}
	return index - 1
}

func xlsxColumnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+index%26)) + name
		index = index/26 - 1
	}
	return name
}
