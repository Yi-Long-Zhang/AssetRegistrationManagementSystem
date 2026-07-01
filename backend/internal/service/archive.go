package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"
)

type TicketArchiveData struct {
	Ticket model.Ticket
}

type TicketArchiver interface {
	Generate(ctx context.Context, data TicketArchiveData, templatePath, archiveDir, libreOfficeBin string) (archiveNo, archivePath string, err error)
}

type LibreOfficeTicketArchiver struct{}

func (LibreOfficeTicketArchiver) Generate(ctx context.Context, data TicketArchiveData, templatePath, archiveDir, libreOfficeBin string) (string, string, error) {
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", "", err
	}
	archiveNo := fmt.Sprintf("ITCFG-%s-%06d", time.Now().Format("20060102"), data.Ticket.ID)
	data.Ticket.ArchiveNo = archiveNo
	workDir, err := os.MkdirTemp(archiveDir, ".archive-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(workDir)

	docxPath := filepath.Join(workDir, archiveNo+".docx")
	if err := fillTicketDocx(templatePath, docxPath, ticketArchiveValues(data.Ticket)); err != nil {
		return "", "", err
	}
	cmd := exec.CommandContext(ctx, libreOfficeBin, "--headless", "--convert-to", "pdf", "--outdir", workDir, docxPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("LibreOffice PDF 转换失败: %v: %s", err, strings.TrimSpace(string(output)))
	}
	pdfPath := filepath.Join(workDir, archiveNo+".pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		return "", "", fmt.Errorf("PDF 文件未生成: %w", err)
	}
	finalPath := filepath.Join(archiveDir, archiveNo+".pdf")
	if err := os.Rename(pdfPath, finalPath); err != nil {
		return "", "", err
	}
	return archiveNo, finalPath, nil
}

func fillTicketDocx(templatePath, outPath string, values map[string]string) error {
	var raw []byte
	var err error
	if templatePath != "" {
		raw, err = os.ReadFile(templatePath)
	}
	if err != nil || len(raw) == 0 {
		raw, err = defaultTicketTemplate()
		if err != nil {
			return err
		}
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return err
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return err
		}
		if strings.HasPrefix(file.Name, "word/") && strings.HasSuffix(file.Name, ".xml") {
			text := string(content)
			for key, value := range values {
				text = strings.ReplaceAll(text, "{{"+key+"}}", xmlEscape(value))
			}
			content = []byte(text)
		}
		header := file.FileHeader
		w, err := writer.CreateHeader(&header)
		if err != nil {
			return err
		}
		if _, err := w.Write(content); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(outPath, body.Bytes(), 0o644)
}

func ticketArchiveValues(ticket model.Ticket) map[string]string {
	applicant := ticket.Applicant.Name
	if applicant == "" {
		applicant = ticket.Applicant.Username
	}
	records := make([]model.TicketRecord, len(ticket.Records))
	copy(records, ticket.Records)
	sort.Slice(records, func(i, j int) bool { return records[i].CreatedAt.Before(records[j].CreatedAt) })
	workflow := make([]model.TicketWorkflowStep, len(ticket.WorkflowSteps))
	copy(workflow, ticket.WorkflowSteps)
	sort.Slice(workflow, func(i, j int) bool { return workflow[i].SortOrder < workflow[j].SortOrder })
	return map[string]string{
		"ArchiveNo":        ticket.ArchiveNo,
		"Department":       ticket.Applicant.Department,
		"Applicant":        applicant,
		"ApplyTime":        formatTime(ticket.CreatedAt),
		"Title":            ticket.Title,
		"TicketType":       string(ticket.Type),
		"DeviceType":       ticket.DeviceType,
		"DeviceName":       ticket.DeviceName,
		"IPAddress":        ticket.IPAddress,
		"OpenPorts":        ticket.OpenPorts,
		"RunningServices":  ticket.RunningServices,
		"AppVersion":       ticket.AppVersion,
		"Manufacturer":     ticket.Manufacturer,
		"Antivirus":        ticket.Antivirus,
		"Reason":           ticket.Description,
		"ChangeContent":    ticket.ChangeContent,
		"Impact":           ticket.Impact,
		"ExecutionRecord":  ticket.Result,
		"AcceptanceResult": ticket.AcceptanceResult,
		"Remark":           ticket.Remark,
		"WorkflowRecords":  workflowText(workflow),
		"TimelineRecords":  recordText(records),
	}
}

func workflowText(steps []model.TicketWorkflowStep) string {
	var lines []string
	for _, step := range steps {
		actor := ""
		if step.Actor != nil {
			actor = step.Actor.Name
			if actor == "" {
				actor = step.Actor.Username
			}
		}
		lines = append(lines, fmt.Sprintf("%s：%s %s %s", step.Name, step.Status, actor, formatTimePtr(step.ActedAt)))
	}
	return strings.Join(lines, "\n")
}

func recordText(records []model.TicketRecord) string {
	var lines []string
	for _, record := range records {
		actor := record.Actor.Name
		if actor == "" {
			actor = record.Actor.Username
		}
		lines = append(lines, fmt.Sprintf("%s %s %s：%s", formatTime(record.CreatedAt), actor, record.Action, record.Remark))
	}
	return strings.Join(lines, "\n")
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}

func xmlEscape(value string) string {
	return html.EscapeString(value)
}

func defaultTicketTemplate() ([]byte, error) {
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>IT配置变更申请表</w:t></w:r></w:p>
<w:p><w:r><w:t>NO：{{ArchiveNo}}</w:t></w:r></w:p>
<w:p><w:r><w:t>部门：{{Department}}  申请人：{{Applicant}}  申请时间：{{ApplyTime}}</w:t></w:r></w:p>
<w:p><w:r><w:t>设备类型：{{DeviceType}}</w:t></w:r></w:p>
<w:p><w:r><w:t>设备名称或IP地址：{{DeviceName}} / {{IPAddress}}</w:t></w:r></w:p>
<w:p><w:r><w:t>开放端口：{{OpenPorts}}</w:t></w:r></w:p>
<w:p><w:r><w:t>运行服务/应用：{{RunningServices}}</w:t></w:r></w:p>
<w:p><w:r><w:t>应用版本：{{AppVersion}}  厂商：{{Manufacturer}}  防病毒软件：{{Antivirus}}</w:t></w:r></w:p>
<w:p><w:r><w:t>申请原因：{{Reason}}</w:t></w:r></w:p>
<w:p><w:r><w:t>配置变更内容：{{ChangeContent}}</w:t></w:r></w:p>
<w:p><w:r><w:t>配置变更对其它系统的影响：{{Impact}}</w:t></w:r></w:p>
<w:p><w:r><w:t>审批记录：{{WorkflowRecords}}</w:t></w:r></w:p>
<w:p><w:r><w:t>配置变更记录：{{ExecutionRecord}}</w:t></w:r></w:p>
<w:p><w:r><w:t>配置变更验证：{{AcceptanceResult}}</w:t></w:r></w:p>
<w:p><w:r><w:t>备注：{{Remark}}</w:t></w:r></w:p>
<w:sectPr/></w:body></w:document>`
	return makeDocx(doc)
}

func makeDocx(documentXML string) ([]byte, error) {
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   documentXML,
	}
	for name, content := range files {
		w, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}
