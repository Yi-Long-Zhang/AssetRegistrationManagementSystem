package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// 许可证导入列（规范表头）。
const (
	licenseColName       = "软件名"
	licenseColVendor     = "厂商"
	licenseColType       = "类型"
	licenseColKey        = "许可证密钥"
	licenseColTotalSeats = "授权数量"
	licenseColUsedSeats  = "已用数量"
	licenseColExpire     = "到期日"
	licenseColPurchase   = "购买日期"
	licenseColAsset      = "关联资产"
	licenseColRemark     = "备注"
)

// licenseImportTemplateHeaders 导入模板表头（含密钥列，新建行必填）。
var licenseImportTemplateHeaders = []string{
	licenseColName, licenseColVendor, licenseColType, licenseColKey, licenseColTotalSeats,
	licenseColUsedSeats, licenseColExpire, licenseColPurchase, licenseColAsset, licenseColRemark,
}

// licenseExportHeaders 导出表头（不含密钥明文，密钥仅在详情 reveal 时解密展示）。
var licenseExportHeaders = []string{
	licenseColName, licenseColVendor, licenseColType, licenseColTotalSeats,
	licenseColUsedSeats, licenseColExpire, licenseColPurchase, licenseColAsset, licenseColRemark,
}

var licenseHeaderAliases = map[string]string{
	"软件名":           licenseColName,
	"软件名称":          licenseColName,
	"softwareName":  licenseColName,
	"software_name": licenseColName,
	"name":          licenseColName,
	"厂商":            licenseColVendor,
	"供应商":           licenseColVendor,
	"vendor":        licenseColVendor,
	"类型":            licenseColType,
	"licenseType":   licenseColType,
	"license_type":  licenseColType,
	"type":          licenseColType,
	"许可证密钥":         licenseColKey,
	"密钥":            licenseColKey,
	"licenseKey":    licenseColKey,
	"license_key":   licenseColKey,
	"key":           licenseColKey,
	"授权数量":          licenseColTotalSeats,
	"授权数":           licenseColTotalSeats,
	"totalSeats":    licenseColTotalSeats,
	"total_seats":   licenseColTotalSeats,
	"已用数量":          licenseColUsedSeats,
	"已用数":           licenseColUsedSeats,
	"usedSeats":     licenseColUsedSeats,
	"used_seats":    licenseColUsedSeats,
	"到期日":           licenseColExpire,
	"到期日期":          licenseColExpire,
	"expireDate":    licenseColExpire,
	"expire_date":   licenseColExpire,
	"购买日期":          licenseColPurchase,
	"采购日期":          licenseColPurchase,
	"purchaseDate":  licenseColPurchase,
	"purchase_date": licenseColPurchase,
	"关联资产":          licenseColAsset,
	"资产":            licenseColAsset,
	"asset":         licenseColAsset,
	"hostname":      licenseColAsset,
	"备注":            licenseColRemark,
	"remark":        licenseColRemark,
}

// LicenseImportTemplateHeaders 返回导入模板表头（含密钥列），供模板下载使用。
func LicenseImportTemplateHeaders() []string {
	return append([]string(nil), licenseImportTemplateHeaders...)
}

// LicenseExportHeaders 返回导出表头（不含密钥明文）。
func LicenseExportHeaders() []string {
	return append([]string(nil), licenseExportHeaders...)
}

// SoftwareLicenseService 软件许可证台账（许可证密钥 AES-GCM 加密存储）。
type SoftwareLicenseService struct {
	DB            *gorm.DB
	EncryptionKey string
}

func NewSoftwareLicenseService(db *gorm.DB, encryptionKey string) *SoftwareLicenseService {
	return &SoftwareLicenseService{DB: db, EncryptionKey: encryptionKey}
}

// List 返回许可证列表（不含密钥明文）。
func (s *SoftwareLicenseService) List() ([]model.SoftwareLicense, error) {
	var list []model.SoftwareLicense
	err := s.DB.Preload("Asset").Order("id desc").Find(&list).Error
	return list, err
}

// Get 返回单个许可证（不含密钥明文）。
func (s *SoftwareLicenseService) Get(id uint) (model.SoftwareLicense, error) {
	var lic model.SoftwareLicense
	err := s.DB.Preload("Asset").First(&lic, id).Error
	return lic, err
}

// Create 加密许可证密钥后创建。
func (s *SoftwareLicenseService) Create(lic *model.SoftwareLicense, plainKey string) error {
	if plainKey == "" {
		return errors.New("license key is required")
	}
	enc, err := EncryptString(plainKey, s.EncryptionKey)
	if err != nil {
		return err
	}
	lic.LicenseKey = enc
	lic.Encrypted = true
	return s.DB.Create(lic).Error
}

// Update 更新许可证元信息；plainKey 非空时重新加密存储。
func (s *SoftwareLicenseService) Update(id uint, lic *model.SoftwareLicense, plainKey string) error {
	var existing model.SoftwareLicense
	if err := s.DB.First(&existing, id).Error; err != nil {
		return err
	}
	existing.Name = lic.Name
	existing.Vendor = lic.Vendor
	existing.Type = lic.Type
	existing.TotalSeats = lic.TotalSeats
	existing.UsedSeats = lic.UsedSeats
	existing.ExpireDate = lic.ExpireDate
	existing.PurchaseDate = lic.PurchaseDate
	existing.AssetID = lic.AssetID
	existing.Remark = lic.Remark
	if plainKey != "" {
		enc, err := EncryptString(plainKey, s.EncryptionKey)
		if err != nil {
			return err
		}
		existing.LicenseKey = enc
		existing.Encrypted = true
	}
	return s.DB.Save(&existing).Error
}

// Delete 删除许可证（软删除）。
func (s *SoftwareLicenseService) Delete(id uint) error {
	return s.DB.Delete(&model.SoftwareLicense{}, id).Error
}

// Reveal 解密并返回明文许可证密钥；无密钥的许可证返回空明文。
func (s *SoftwareLicenseService) Reveal(id uint) (model.SoftwareLicense, string, error) {
	var lic model.SoftwareLicense
	if err := s.DB.First(&lic, id).Error; err != nil {
		return lic, "", err
	}
	if !lic.Encrypted || lic.LicenseKey == "" {
		return lic, "", nil
	}
	plain, err := DecryptString(lic.LicenseKey, s.EncryptionKey)
	if err != nil {
		return lic, "", err
	}
	return lic, plain, nil
}

// ImportRows 批量导入许可证：按「软件名+厂商」匹配已有记录则更新（密钥非空时重加密），
// 否则新建（密钥必填）。任一行出错整批回滚，返回 created/updated。
// records 为 CSV/XLSX 解析出的原始行，会自动跳过表头前的说明行。
func (s *SoftwareLicenseService) ImportRows(records [][]string) (created, updated int, err error) {
	records = normalizeLicenseImportRecords(records)
	if len(records) < 2 {
		return 0, 0, errors.New("至少需要表头和一行许可证数据")
	}
	headerIndex := licenseImportHeaderIndex(records[0])
	var rowErrors []string
	txErr := s.DB.Transaction(func(tx *gorm.DB) error {
		for i, row := range records[1:] {
			if csvRowEmpty(row) {
				continue
			}
			lic, plainKey, err := licenseFromImportRow(tx, headerIndex, row)
			if err != nil {
				rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, err.Error()))
				continue
			}
			var existing model.SoftwareLicense
			query := tx.Where("name = ?", lic.Name)
			if strings.TrimSpace(lic.Vendor) != "" {
				query = query.Where("vendor = ?", strings.TrimSpace(lic.Vendor))
			} else {
				query = query.Where("(vendor = '' OR vendor IS NULL)")
			}
			err = query.First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if plainKey == "" {
					rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, "新增许可证必须填写许可证密钥"))
					continue
				}
				enc, encErr := EncryptString(plainKey, s.EncryptionKey)
				if encErr != nil {
					rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, encErr.Error()))
					continue
				}
				lic.LicenseKey = enc
				lic.Encrypted = true
				if err := tx.Create(&lic).Error; err != nil {
					rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, err.Error()))
					continue
				}
				created++
				continue
			}
			if err != nil {
				rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, err.Error()))
				continue
			}
			existing.Name = lic.Name
			existing.Vendor = lic.Vendor
			existing.Type = lic.Type
			existing.TotalSeats = lic.TotalSeats
			existing.UsedSeats = lic.UsedSeats
			existing.ExpireDate = lic.ExpireDate
			existing.PurchaseDate = lic.PurchaseDate
			existing.AssetID = lic.AssetID
			existing.Remark = lic.Remark
			if plainKey != "" {
				enc, encErr := EncryptString(plainKey, s.EncryptionKey)
				if encErr != nil {
					rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, encErr.Error()))
					continue
				}
				existing.LicenseKey = enc
				existing.Encrypted = true
			}
			if err := tx.Save(&existing).Error; err != nil {
				rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, err.Error()))
				continue
			}
			updated++
		}
		if len(rowErrors) > 0 {
			return errors.New(strings.Join(rowErrors, "; "))
		}
		return nil
	})
	if txErr != nil {
		return 0, 0, txErr
	}
	return created, updated, nil
}

// licenseFromImportRow 按表头索引解析一行导入数据。
func licenseFromImportRow(tx *gorm.DB, headerIndex map[string]int, row []string) (model.SoftwareLicense, string, error) {
	get := func(canonical string) string {
		idx, ok := headerIndex[canonical]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}
	lic := model.SoftwareLicense{
		Name:   get(licenseColName),
		Vendor: get(licenseColVendor),
		Type:   model.NormalizeLicenseType(get(licenseColType)),
		Remark: get(licenseColRemark),
	}
	plainKey := get(licenseColKey)
	lic.Name = strings.TrimSpace(lic.Name)
	if lic.Name == "" {
		return model.SoftwareLicense{}, "", errors.New("软件名不能为空")
	}
	if total, err := strconv.Atoi(get(licenseColTotalSeats)); err == nil {
		if total < 0 {
			return model.SoftwareLicense{}, "", errors.New("授权数量不能为负数")
		}
		lic.TotalSeats = total
	}
	if used, err := strconv.Atoi(get(licenseColUsedSeats)); err == nil {
		if used < 0 {
			return model.SoftwareLicense{}, "", errors.New("已用数量不能为负数")
		}
		lic.UsedSeats = used
	}
	if value := get(licenseColExpire); value != "" {
		parsed, err := parseLicenseDate(value)
		if err != nil {
			return model.SoftwareLicense{}, "", fmt.Errorf("到期日 %s", err.Error())
		}
		lic.ExpireDate = &parsed
	}
	if value := get(licenseColPurchase); value != "" {
		parsed, err := parseLicenseDate(value)
		if err != nil {
			return model.SoftwareLicense{}, "", fmt.Errorf("购买日期 %s", err.Error())
		}
		lic.PurchaseDate = &parsed
	}
	if asset := get(licenseColAsset); asset != "" {
		var matched model.Asset
		err := tx.Where("hostname = ? OR ip = ?", asset, asset).First(&matched).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.SoftwareLicense{}, "", fmt.Errorf("关联资产不存在: %s", asset)
		}
		if err != nil {
			return model.SoftwareLicense{}, "", err
		}
		lic.AssetID = &matched.ID
	}
	return lic, plainKey, nil
}

func licenseImportHeaderIndex(headers []string) map[string]int {
	index := map[string]int{}
	for i, header := range headers {
		normalized := normalizeHeader(header)
		if canonical, ok := licenseHeaderAliases[normalized]; ok {
			normalized = canonical
		}
		index[normalized] = i
	}
	return index
}

// normalizeLicenseImportRecords 跳过表头前的说明行，返回从含「软件名」表头行开始的记录。
func normalizeLicenseImportRecords(records [][]string) [][]string {
	for i, row := range records {
		headerIndex := licenseImportHeaderIndex(row)
		if _, ok := headerIndex[licenseColName]; ok {
			return records[i:]
		}
	}
	return records
}

func parseLicenseDate(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "2006/01/02", "2006.01.02", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("日期格式应为 YYYY-MM-DD")
}

func normalizeHeader(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "\ufeff")
}

func csvRowEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
