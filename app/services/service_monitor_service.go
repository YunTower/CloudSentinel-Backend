package services

import (
	"crypto/tls"
	"errors"
	"fmt"
	"goravel/app/models"
	"goravel/app/repositories"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/goravel/framework/facades"
)

var (
	serviceMonitorOnce     sync.Once
	serviceMonitorInstance *ServiceMonitorService
)

type ServiceMonitorService struct {
	mu      sync.Mutex
	stopChs map[uint]chan struct{}
}

func GetServiceMonitorService() *ServiceMonitorService {
	serviceMonitorOnce.Do(func() {
		serviceMonitorInstance = &ServiceMonitorService{
			stopChs: make(map[uint]chan struct{}),
		}
	})
	return serviceMonitorInstance
}

func (s *ServiceMonitorService) StartAll() {
	repo := repositories.GetServiceMonitorRepository()
	monitors, err := repo.GetEnabled()
	if err != nil {
		facades.Log().Errorf("加载服务监测失败: %v", err)
		return
	}
	for _, m := range monitors {
		s.Start(m)
	}
}

func (s *ServiceMonitorService) Start(m *models.ServiceMonitor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.stopChs[m.ID]; ok {
		close(ch)
	}
	interval := m.Interval
	if interval < 10 {
		interval = 10
	}
	stopCh := make(chan struct{})
	s.stopChs[m.ID] = stopCh

	go func(monitor *models.ServiceMonitor, stop chan struct{}) {
		s.runCheck(monitor)
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runCheck(monitor)
			case <-stop:
				return
			}
		}
	}(m, stopCh)
}

func (s *ServiceMonitorService) Stop(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.stopChs[id]; ok {
		close(ch)
		delete(s.stopChs, id)
	}
}

func (s *ServiceMonitorService) runCheck(m *models.ServiceMonitor) {
	// 如果配置了server_ids，则分发到Agent；面板也直接检查
	if len(m.ServerIDs) > 0 {
		wsService := GetWebSocketService()
		manager := wsService.GetManager()
		payload := map[string]interface{}{
			"command": "service_check",
			"data": map[string]interface{}{
				"monitor_id":    m.ID,
				"type":          m.Type,
				"target":        m.Target,
				"port":          m.Port,
				"timeout":       m.Timeout,
				"expect_status": m.ExpectStatus,
				"expect_body":   m.ExpectBody,
			},
		}
		for _, sid := range m.ServerIDs {
			_ = manager.SendToAgent(sid, payload)
		}
		return
	}
	// 面板直接检查
	s.doCheck(m.ID, m.Type, m.Target, m.Port, m.Timeout, m.ExpectStatus, m.ExpectBody)
}

// 执行检查
func (s *ServiceMonitorService) doCheck(id uint, typ, target string, port, timeout, expectStatus int, expectBody string) {
	start := time.Now()
	var checkErr error

	if timeout <= 0 {
		timeout = 10
	}

	switch typ {
	case "http", "https":
		checkErr = checkHTTP(target, timeout, expectStatus, expectBody)
	case "tcp":
		checkErr = checkTCP(target, port, timeout)
	case "udp":
		checkErr = checkUDP(target, port, timeout)
	default:
		checkErr = fmt.Errorf("unknown type: %s", typ)
	}

	elapsed := int(time.Since(start).Milliseconds())
	now := time.Now()

	status := "up"
	if checkErr != nil {
		var netErr net.Error
		if errors.As(checkErr, &netErr) && netErr.Timeout() {
			status = "slow" // yellow – connected but timed out waiting for response
		} else {
			status = "down" // red – cannot connect
		}
	}

	repo := repositories.GetServiceMonitorRepository()
	_ = repo.Update(id, map[string]interface{}{
		"status":        status,
		"response_time": elapsed,
		"last_check_at": now,
	})

	// 保存历史记录并修剪到最近的24条记录
	histRepo := repositories.GetServiceMonitorHistoryRepository()
	histEntry := &models.ServiceMonitorHistory{
		MonitorID:    id,
		Status:       status,
		ResponseTime: elapsed,
		CheckedAt:    now,
	}
	if saveErr := histRepo.Save(histEntry); saveErr == nil {
		go histRepo.PruneOld(id, 24)
	}

	GetWebSocketService().GetManager().BroadcastToFrontend(map[string]interface{}{
		"type": "service_monitor_update",
		"data": map[string]interface{}{
			"id":            id,
			"status":        status,
			"response_time": elapsed,
			"last_check_at": now.Format(time.RFC3339),
			"history_entry": map[string]interface{}{
				"status":        status,
				"response_time": elapsed,
				"checked_at":    now.Format(time.RFC3339),
			},
		},
	})

	if checkErr != nil {
		facades.Log().Warningf("服务监测 [%d] 检测失败: %v", id, checkErr)
	}
}

// 处理Agent结果
func (s *ServiceMonitorService) HandleAgentResult(data map[string]interface{}) {
	monitorID, _ := data["monitor_id"].(float64)
	status, _ := data["status"].(string)
	responseTime, _ := data["response_time"].(float64)
	if monitorID == 0 || status == "" {
		return
	}
	id := uint(monitorID)
	elapsed := int(responseTime)
	now := time.Now()

	repo := repositories.GetServiceMonitorRepository()
	_ = repo.Update(id, map[string]interface{}{
		"status":        status,
		"response_time": elapsed,
		"last_check_at": now,
	})

	// 保存历史记录并修剪到最近的24条记录
	histRepo := repositories.GetServiceMonitorHistoryRepository()
	histEntry := &models.ServiceMonitorHistory{
		MonitorID:    id,
		Status:       status,
		ResponseTime: elapsed,
		CheckedAt:    now,
	}
	if saveErr := histRepo.Save(histEntry); saveErr == nil {
		go histRepo.PruneOld(id, 24)
	}

	GetWebSocketService().GetManager().BroadcastToFrontend(map[string]interface{}{
		"type": "service_monitor_update",
		"data": map[string]interface{}{
			"id":            id,
			"status":        status,
			"response_time": elapsed,
			"last_check_at": now.Format(time.RFC3339),
			"history_entry": map[string]interface{}{
				"status":        status,
				"response_time": elapsed,
				"checked_at":    now.Format(time.RFC3339),
			},
		},
	})
}

// 检查HTTP服务
func checkHTTP(target string, timeoutSec, expectStatus int, expectBody string) error {
	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}
	resp, err := client.Get(target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查状态码
	if expectStatus > 0 {
		if resp.StatusCode != expectStatus {
			return fmt.Errorf("期望状态码 %d，实际 %d", expectStatus, resp.StatusCode)
		}
	} else if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 检查响应体
	if expectBody != "" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("读取响应体失败: %v", err)
		}
		if !strings.Contains(string(body), expectBody) {
			return fmt.Errorf("响应体不包含期望内容")
		}
	}
	return nil
}

// 检查TCP服务
func checkTCP(host string, port int, timeoutSec int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// 检查UDP服务
func checkUDP(host string, port int, timeoutSec int) error {
	conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:%d", host, port), time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
