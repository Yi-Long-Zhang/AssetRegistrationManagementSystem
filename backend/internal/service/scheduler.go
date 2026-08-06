package service

import (
	"context"
	"log"
	"sync"
	"time"

	"asset-registration-management-system/backend/internal/model"
)

// DiscoveryScheduler 进程内调度器：每分钟检查一次启用的发现规则，
// 到达间隔且无重叠运行时触发扫描；进程重启后按 LastRunAt 自然恢复节奏。
type DiscoveryScheduler struct {
	svc     *DiscoveryService
	mu      sync.Mutex
	running map[uint]bool // 规则 ID -> 是否正在运行（防重叠）
	stop    chan struct{}
	wg      sync.WaitGroup
}

// NewDiscoveryScheduler 创建调度器。
func NewDiscoveryScheduler(svc *DiscoveryService) *DiscoveryScheduler {
	return &DiscoveryScheduler{svc: svc, running: map[uint]bool{}}
}

// Start 启动调度循环（幂等：重复调用仅启动一次）。
func (s *DiscoveryScheduler) Start() {
	if s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	s.wg.Add(1)
	go s.loop()
	log.Printf("discovery scheduler started")
}

// Stop 停止调度循环并等待进行中的任务结束。
func (s *DiscoveryScheduler) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	s.wg.Wait()
	s.stop = nil
}

func (s *DiscoveryScheduler) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *DiscoveryScheduler) tick() {
	var rules []model.DiscoveryRule
	if err := s.svc.DB.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		log.Printf("discovery scheduler: load rules: %v", err)
		return
	}
	now := time.Now()
	for i := range rules {
		rule := rules[i]
		interval := time.Duration(rule.IntervalMinutes) * time.Minute
		if interval <= 0 {
			interval = 60 * time.Minute
		}
		if rule.LastRunAt != nil && rule.LastRunAt.Add(interval).After(now) {
			continue // 未到调度时间
		}
		s.mu.Lock()
		if s.running[rule.ID] {
			s.mu.Unlock()
			continue
		}
		s.running[rule.ID] = true
		s.mu.Unlock()

		s.wg.Add(1)
		go func(r model.DiscoveryRule) {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.running, r.ID)
				s.mu.Unlock()
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.svc.Config.Discovery.ScanTimeoutSec)*time.Second+30*time.Second)
			defer cancel()
			if _, err := s.svc.RunRule(ctx, r.ID, "schedule"); err != nil {
				log.Printf("discovery scheduler: rule %d run failed: %v", r.ID, err)
			}
		}(rule)
	}
}
