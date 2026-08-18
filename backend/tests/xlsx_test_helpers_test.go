package tests

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
)

func buildTestXLSX(rows [][]string) []byte {
	var body bytes.Buffer
	zipWriter := zip.NewWriter(&body)
	sheet, _ := zipWriter.Create("xl/worksheets/sheet1.xml")
	_, _ = sheet.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`))
	for rowIndex, row := range rows {
		_, _ = sheet.Write([]byte(fmt.Sprintf(`<row r="%d">`, rowIndex+1)))
		for colIndex, value := range row {
			cellRef := fmt.Sprintf("%s%d", excelColumnName(colIndex), rowIndex+1)
			_, _ = sheet.Write([]byte(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, cellRef, value)))
		}
		_, _ = sheet.Write([]byte(`</row>`))
	}
	_, _ = sheet.Write([]byte(`</sheetData></worksheet>`))
	_ = zipWriter.Close()
	return body.Bytes()
}

func zipContains(content []byte, term string) bool {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return false
	}
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err == nil && bytes.Contains(data, []byte(term)) {
			return true
		}
	}
	return false
}

func readXLSXRows(reader io.ReaderAt, size int64) ([][]string, error) {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}
	sharedStrings, err := readSharedStrings(zipReader)
	if err != nil {
		return nil, err
	}
	for _, file := range zipReader.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return parseSheetRows(rc, sharedStrings)
	}
	return nil, fmt.Errorf("sheet1.xml not found")
}

func readSharedStrings(zipReader *zip.Reader) ([]string, error) {
	for _, file := range zipReader.File {
		if file.Name != "xl/sharedStrings.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		var sst struct {
			Items []struct {
				Text string `xml:"t"`
			} `xml:"si"`
		}
		if err := xml.NewDecoder(rc).Decode(&sst); err != nil {
			return nil, err
		}
		values := make([]string, 0, len(sst.Items))
		for _, item := range sst.Items {
			values = append(values, item.Text)
		}
		return values, nil
	}
	return nil, nil
}

func parseSheetRows(reader io.Reader, sharedStrings []string) ([][]string, error) {
	decoder := xml.NewDecoder(reader)
	rows := [][]string{}
	var current []string
	inCell := false
	cellType := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "row":
				current = []string{}
			case "c":
				inCell = true
				cellType = attrValue(element.Attr, "t")
			case "v", "t":
				if inCell {
					var value string
					if err := decoder.DecodeElement(&value, &element); err != nil {
						return nil, err
					}
					if cellType == "s" {
						index := 0
						_, _ = fmt.Sscanf(value, "%d", &index)
						if index >= 0 && index < len(sharedStrings) {
							value = sharedStrings[index]
						}
					}
					current = append(current, value)
				}
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "c":
				inCell = false
				cellType = ""
			case "row":
				rows = append(rows, current)
			}
		}
	}
	return rows, nil
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func excelColumnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+index%26)) + name
		index = index/26 - 1
	}
	return name
}

func itoa(id uint) string {
	return json.Number(fmtUint(id)).String()
}

func fmtUint(id uint) string {
	return strconvFormatUint(uint64(id))
}

func strconvFormatUint(id uint64) string {
	const digits = "0123456789"
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = digits[id%10]
		id /= 10
	}
	return string(buf[i:])
}
