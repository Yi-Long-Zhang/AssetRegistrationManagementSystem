package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type metricKey struct {
	Method string
	Path   string
	Status int
}

type metricValue struct {
	Count       uint64
	DurationSum float64
}

type HTTPMetrics struct {
	mu       sync.Mutex
	requests map[metricKey]metricValue
}

func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{requests: map[metricKey]metricValue{}}
}

func requestObservability(metrics *HTTPMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		requestID := cleanRequestID(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		duration := time.Since(started)
		key := metricKey{Method: c.Request.Method, Path: path, Status: c.Writer.Status()}
		metrics.mu.Lock()
		value := metrics.requests[key]
		value.Count++
		value.DurationSum += duration.Seconds()
		metrics.requests[key] = value
		metrics.mu.Unlock()
		slog.Info("http_request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", duration.Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

func cleanRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return ""
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '-' && char != '_' {
			return ""
		}
	}
	return value
}

func generateRequestID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(value)
}

func (h *Handler) Metrics(c *gin.Context) {
	var output strings.Builder
	output.WriteString("# TYPE arms_http_requests_total counter\n")
	output.WriteString("# TYPE arms_http_request_duration_seconds summary\n")
	h.metrics.mu.Lock()
	keys := make([]metricKey, 0, len(h.metrics.requests))
	for key := range h.metrics.requests {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j])
	})
	for _, key := range keys {
		value := h.metrics.requests[key]
		labels := fmt.Sprintf("method=%q,path=%q,status=%q", key.Method, key.Path, strconv.Itoa(key.Status))
		fmt.Fprintf(&output, "arms_http_requests_total{%s} %d\n", labels, value.Count)
		fmt.Fprintf(&output, "arms_http_request_duration_seconds_sum{%s} %f\n", labels, value.DurationSum)
		fmt.Fprintf(&output, "arms_http_request_duration_seconds_count{%s} %d\n", labels, value.Count)
	}
	h.metrics.mu.Unlock()

	var taskCounts []struct {
		Status model.BackgroundTaskStatus
		Count  int64
	}
	_ = h.db.Model(&model.BackgroundTask{}).Select("status, COUNT(*) AS count").Group("status").Scan(&taskCounts).Error
	output.WriteString("# TYPE arms_background_tasks gauge\n")
	for _, item := range taskCounts {
		fmt.Fprintf(&output, "arms_background_tasks{status=%q} %d\n", item.Status, item.Count)
	}
	var sessions int64
	_ = h.db.Model(&model.AuthSession{}).
		Where("revoked_at IS NULL AND expires_at > ?", time.Now()).Count(&sessions).Error
	output.WriteString("# TYPE arms_auth_sessions_active gauge\n")
	fmt.Fprintf(&output, "arms_auth_sessions_active %d\n", sessions)
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(output.String()))
}

type readinessCheck struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

func readiness(cfg config.Config, db *gorm.DB, tasks *service.TaskManager) (map[string]readinessCheck, bool) {
	checks := map[string]readinessCheck{}
	allOK := true
	sqlDB, err := db.DB()
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = sqlDB.PingContext(ctx)
		cancel()
	}
	checks["database"] = checkResult(err)
	allOK = allOK && err == nil

	for name, path := range map[string]string{
		"database_dir": filepath.Dir(cfg.Storage.DatabasePath),
		"attachments":  cfg.Storage.AttachmentDir,
		"archives":     cfg.Storage.TicketArchiveDir,
		"backups":      cfg.Storage.BackupDir,
	} {
		err = checkWritableDirectory(path)
		checks[name] = checkResult(err)
		allOK = allOK && err == nil
	}
	err = checkBinary(cfg.Storage.LibreOfficeBin)
	checks["libreoffice"] = checkResult(err)
	allOK = allOK && err == nil
	nmapBinary := cfg.Discovery.NmapBin
	if nmapBinary == "" {
		nmapBinary = "nmap"
	}
	err = checkBinary(nmapBinary)
	checks["nmap"] = checkResult(err)
	allOK = allOK && err == nil

	kinds := tasks.RegisteredKinds()
	if len(kinds) == 0 {
		err = errors.New("no task handlers registered")
	} else {
		err = nil
	}
	checks["schedulers"] = checkResult(err)
	allOK = allOK && err == nil
	return checks, allOK
}

func checkResult(err error) readinessCheck {
	if err != nil {
		return readinessCheck{OK: false, Detail: err.Error()}
	}
	return readinessCheck{OK: true}
}

func checkWritableDirectory(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(path, ".ready-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}

func checkBinary(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("binary is not configured")
	}
	if strings.ContainsAny(name, `/\`) {
		if info, err := os.Stat(name); err != nil || info.IsDir() {
			return fmt.Errorf("binary not found: %s", name)
		}
		return nil
	}
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("binary not found: %s", name)
	}
	return nil
}
