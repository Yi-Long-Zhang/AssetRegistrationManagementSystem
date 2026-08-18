package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

type fakeArchiver struct{}

func (fakeArchiver) Generate(_ context.Context, data service.TicketArchiveData, _ string, archiveDir string, _ string) (string, string, error) {
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", "", err
	}
	archiveNo := fmt.Sprintf("ITCFG-TEST-%06d", data.Ticket.ID)
	archivePath := filepath.Join(archiveDir, archiveNo+".pdf")
	if err := os.WriteFile(archivePath, []byte("%PDF-1.4\n% test archive\n"), 0o644); err != nil {
		return "", "", err
	}
	return archiveNo, archivePath, nil
}

type fakeMailSender struct{}

func (fakeMailSender) Send(mailConfig model.MailConfig, password string, message service.MailMessage) error {
	if mailConfig.SMTPHost != "smtp.example.com" {
		return fmt.Errorf("invalid smtp host")
	}
	if password != "smtp-secret" {
		return fmt.Errorf("invalid smtp password")
	}
	if len(message.To) == 0 {
		return fmt.Errorf("empty recipients")
	}
	return nil
}

type fakeADClient struct{}

func (fakeADClient) Test(_ model.ADConfig, bindPassword string) error {
	if bindPassword != "bind-secret" {
		return fmt.Errorf("invalid bind password")
	}
	return nil
}

func (fakeADClient) LookupUser(_ model.ADConfig, _ string, username string) (service.ADUserInfo, error) {
	if username != "zhangsan" {
		return service.ADUserInfo{}, fmt.Errorf("not found")
	}
	return service.ADUserInfo{
		Username:    "zhangsan",
		DN:          "cn=zhangsan,dc=example,dc=com",
		DisplayName: "张三",
		Email:       "zhangsan@example.com",
		Department:  "运维部",
	}, nil
}

func (fakeADClient) Authenticate(cfg model.ADConfig, bindPassword, username, password string) (service.ADUserInfo, error) {
	if password != "ad-password" {
		return service.ADUserInfo{}, fmt.Errorf("invalid credentials")
	}
	return fakeADClient{}.LookupUser(cfg, bindPassword, username)
}
