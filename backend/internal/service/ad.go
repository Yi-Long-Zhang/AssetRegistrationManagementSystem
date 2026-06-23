package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/go-ldap/ldap/v3"
)

type ADUserInfo struct {
	Username    string `json:"username"`
	DN          string `json:"dn"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Department  string `json:"department"`
}

type ADClient interface {
	Test(config model.ADConfig, bindPassword string) error
	LookupUser(config model.ADConfig, bindPassword, username string) (ADUserInfo, error)
	Authenticate(config model.ADConfig, bindPassword, username, password string) (ADUserInfo, error)
}

type LDAPADClient struct{}

func (LDAPADClient) Test(config model.ADConfig, bindPassword string) error {
	conn, err := ldap.DialURL(config.LDAPURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Bind(config.BindDN, bindPassword)
}

func (c LDAPADClient) LookupUser(config model.ADConfig, bindPassword, username string) (ADUserInfo, error) {
	conn, err := ldap.DialURL(config.LDAPURL)
	if err != nil {
		return ADUserInfo{}, err
	}
	defer conn.Close()
	if err := conn.Bind(config.BindDN, bindPassword); err != nil {
		return ADUserInfo{}, err
	}
	return searchADUser(conn, config, username)
}

func (c LDAPADClient) Authenticate(config model.ADConfig, bindPassword, username, password string) (ADUserInfo, error) {
	if strings.TrimSpace(password) == "" {
		return ADUserInfo{}, errors.New("password is required")
	}
	conn, err := ldap.DialURL(config.LDAPURL)
	if err != nil {
		return ADUserInfo{}, err
	}
	defer conn.Close()
	if err := conn.Bind(config.BindDN, bindPassword); err != nil {
		return ADUserInfo{}, err
	}
	info, err := searchADUser(conn, config, username)
	if err != nil {
		return ADUserInfo{}, err
	}
	if err := conn.Bind(info.DN, password); err != nil {
		return ADUserInfo{}, err
	}
	return info, nil
}

func searchADUser(conn *ldap.Conn, config model.ADConfig, username string) (ADUserInfo, error) {
	filterTemplate := config.UserFilter
	if strings.TrimSpace(filterTemplate) == "" {
		filterTemplate = "(sAMAccountName=%s)"
	}
	filter := fmt.Sprintf(filterTemplate, ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,
		10,
		false,
		filter,
		[]string{"sAMAccountName", "displayName", "cn", "mail", "department", "distinguishedName"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		return ADUserInfo{}, err
	}
	if len(result.Entries) == 0 {
		return ADUserInfo{}, errors.New("AD user not found")
	}
	entry := result.Entries[0]
	displayName := entry.GetAttributeValue("displayName")
	if displayName == "" {
		displayName = entry.GetAttributeValue("cn")
	}
	sam := entry.GetAttributeValue("sAMAccountName")
	if sam == "" {
		sam = username
	}
	return ADUserInfo{
		Username:    sam,
		DN:          entry.DN,
		DisplayName: displayName,
		Email:       entry.GetAttributeValue("mail"),
		Department:  entry.GetAttributeValue("department"),
	}, nil
}

func EncryptString(plainText, keyText string) (string, error) {
	block, err := aes.NewCipher(encryptionKey(keyText))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func DecryptString(cipherText, keyText string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(encryptionKey(keyText))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext is too short")
	}
	nonce := raw[:gcm.NonceSize()]
	data := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func encryptionKey(keyText string) []byte {
	sum := sha256.Sum256([]byte(keyText))
	return sum[:]
}

func ApplyADInfo(user *model.User, info ADUserInfo) {
	now := time.Now()
	user.Username = info.Username
	user.Name = defaultString(info.DisplayName, info.Username)
	user.DisplayName = info.DisplayName
	user.Email = info.Email
	user.Department = info.Department
	user.ADDN = info.DN
	user.AuthSource = "ad"
	user.LastLoginAt = &now
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
