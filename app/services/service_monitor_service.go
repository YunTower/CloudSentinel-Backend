package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"goravel/app/models"
	"goravel/app/monitorprobe"
	"goravel/app/repositories"
	"goravel/app/utils/secret"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"goravel/app/facades"
)

var (
	serviceMonitorOnce     sync.Once
	serviceMonitorInstance *ServiceMonitorService
)

const (
	serviceMonitorHistoryPreviewLimit = 60
	serviceMonitorHistoryRetention    = 30 * 24 * time.Hour
	serviceMonitorResultRetention     = 90 * 24 * time.Hour
)

type ServiceMonitorService struct {
	mu      sync.Mutex
	stopChs map[uint]chan struct{}
	rounds  *serviceMonitorRounds

	// 状态推进的 per-monitor 互斥锁：commitMonitorStatus 是读-改-写流程，
	// 多探测点并发 finalize / 迟到结果 / 面板直检同时写同一 monitor 时
	// 会互相覆盖 consecutive_failures 等字段，必须串行化。
	statusMu    sync.Mutex
	statusLocks map[uint]*sync.Mutex
}

// lockMonitor 返回该 monitor 的互斥锁解锁函数。
// 注意：多实例部署下此锁仅保护本进程，跨实例部署仍需外部互斥（见 README）。
func (s *ServiceMonitorService) lockMonitor(id uint) func() {
	s.statusMu.Lock()
	if s.statusLocks == nil {
		s.statusLocks = make(map[uint]*sync.Mutex)
	}
	lock, ok := s.statusLocks[id]
	if !ok {
		lock = &sync.Mutex{}
		s.statusLocks[id] = lock
	}
	s.statusMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

type probeCheckResult struct {
	probeID      string
	status       string
	responseTime int
	err          error
}

func GetServiceMonitorService() *ServiceMonitorService {
	serviceMonitorOnce.Do(func() {
		serviceMonitorInstance = &ServiceMonitorService{
			stopChs:     make(map[uint]chan struct{}),
			rounds:      newServiceMonitorRounds(),
			statusLocks: make(map[uint]*sync.Mutex),
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

	// 复制结构体，防止 Restart 后旧 goroutine 仍在 runCheck 中时使用过期指针数据
	monitorCopy := *m
	go func(monitor models.ServiceMonitor, stop chan struct{}) {
		s.runCheck(&monitor)
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runCheck(&monitor)
			case <-stop:
				return
			}
		}
	}(monitorCopy, stopCh)
}

func (s *ServiceMonitorService) Stop(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.stopChs[id]; ok {
		close(ch)
		delete(s.stopChs, id)
	}
	s.rounds.CancelMonitor(id)
}

// StopAll 停止全部监测循环与尚未完成的多 Agent 检测轮次。
func (s *ServiceMonitorService) StopAll() {
	s.mu.Lock()
	for id, ch := range s.stopChs {
		close(ch)
		delete(s.stopChs, id)
	}
	s.mu.Unlock()
	s.rounds.CancelAll()
}

func (s *ServiceMonitorService) runCheck(m *models.ServiceMonitor) {
	if isProtocolMonitorType(m.Type) {
		s.doProtocolCheck(m)
		return
	}
	// 如果配置了 server_ids，则分发到 Agent；所有目标 Agent 都不可用时立即记录失败。
	if len(m.ServerIDs) > 0 {
		wsService := GetWebSocketService()
		manager := wsService.GetManager()
		checkID := uuid.NewString()
		commandID := uuid.NewString()
		timeoutSec := m.Timeout
		if timeoutSec <= 0 {
			timeoutSec = 10
		}
		deadlineAt := time.Now().Add(time.Duration(timeoutSec+2) * time.Second).UTC()
		s.createPendingCheck(m.ID, checkID, commandID, m.ServerIDs, timeoutSec)
		taskData := map[string]interface{}{
			"monitor_id":    m.ID,
			"check_id":      checkID,
			"type":          m.Type,
			"target":        m.Target,
			"port":          m.Port,
			"timeout":       m.Timeout,
			"expect_status": m.ExpectStatus,
			"expect_body":   m.ExpectBody,
			"http_method":   m.HTTPMethod,
			"http_headers":  m.HTTPHeaders,
			"http_body":     m.HTTPBody,
			"deadline_at":   deadlineAt.Format(time.RFC3339Nano),
		}
		payload := map[string]interface{}{
			"command":    "service_check",
			"command_id": commandID,
			"data":       taskData,
		}
		// 命令签名：Agent 端使用面板公钥验签后才执行。
		// 签名失败必须拒绝下发——发送未签名命令对旧版 Agent 是裸奔、
		// 对新版 Agent 是必然失败，宁可本轮判 unknown 也不裸发。
		sig, ts, sigErr := SignAgentCommand(taskData, commandID)
		if sigErr != nil {
			facades.Log().Errorf("服务监测命令签名失败，本轮不下发: monitor_id=%d, error=%v", m.ID, sigErr)
			s.finalizePendingCheck(m.ID, checkID, true)
			return
		}
		payload["sig"] = sig
		payload["sig_ts"] = ts
		sentCount := 0
		queuedCount := 0
		for _, sid := range m.ServerIDs {
			if err := manager.SendToAgent(sid, payload); err != nil {
				facades.Log().Warningf("服务监测 [%d] 下发到 Agent 失败: server_id=%s, error=%v", m.ID, sid, err)
				taskData, _ := payload["data"].(map[string]interface{})
				if _, queueErr := NewAgentTaskService().Enqueue(sid, "service_check", commandID, taskData); queueErr != nil {
					facades.Log().Warningf("服务监测 [%d] 写入 Agent 任务队列失败: server_id=%s, error=%v", m.ID, sid, queueErr)
					s.addPendingAgentResult(m.ID, checkID, sid, "down", 0, err)
				} else {
					queuedCount++
				}
				continue
			}
			sentCount++
		}
		if sentCount+queuedCount == 0 {
			s.finalizePendingCheck(m.ID, checkID, false)
		}
		return
	}
	// 面板直接检查
	s.doCheck(m.ID, m.Type, m.Target, m.Port, m.Timeout, m.ExpectStatus, m.ExpectBody, m.HTTPMethod, m.HTTPHeaders, m.HTTPBody, m.CheckCertExpiry)
}

func isProtocolMonitorType(monitorType string) bool {
	switch monitorType {
	case monitorprobe.TypeAIModel, monitorprobe.TypeMinecraftJava, monitorprobe.TypeMinecraftBedrock:
		return true
	default:
		return false
	}
}

func (s *ServiceMonitorService) doProtocolCheck(m *models.ServiceMonitor) {
	apiKey := ""
	if m.Type == monitorprobe.TypeAIModel {
		var err error
		apiKey, err = secret.DecryptStringWithAppKey(m.AIAPIKeyEncrypted)
		if err != nil {
			s.recordResult(m.ID, "panel", "", monitorprobe.StatusUnknown, 0,
				fmt.Errorf("AI API Key 解密失败: %w", err), nil, "credential_unavailable", nil)
			return
		}
	}
	allowPrivate := monitorAllowsPrivateTargets()
	result := monitorprobe.Check(context.Background(), monitorprobe.Request{
		Type:         m.Type,
		Target:       m.Target,
		Port:         m.Port,
		Timeout:      time.Duration(m.Timeout) * time.Second,
		AllowPrivate: allowPrivate,
		AIFormat:     m.AIAPIFormat,
		AIModel:      m.AIModel,
		AIAPIKey:     apiKey,
	})
	s.recordResult(m.ID, "panel", "", result.Status, result.ResponseTime, result.Error, nil, result.ErrorCode, result.Metadata)
}

// 执行检查
func (s *ServiceMonitorService) doCheck(id uint, typ, target string, port, timeout, expectStatus int, expectBody, httpMethod, httpHeaders, httpBody string, checkCertExpiry bool) {
	start := time.Now()
	var checkErr error
	var certInfo *certCheckInfo

	if timeout <= 0 {
		timeout = 10
	}

	allowPrivate := monitorAllowsPrivateTargets()

	switch typ {
	case "http", "https":
		info, err := checkHTTP(target, timeout, expectStatus, expectBody, httpMethod, httpHeaders, httpBody, typ == "https" && checkCertExpiry)
		checkErr = err
		if typ == "https" && checkCertExpiry && !info.ExpiresAt.IsZero() {
			certInfo = &info
		}
	case "tcp":
		host, defferedErr := probeTargetHost("tcp", target, 0, allowPrivate)
		if defferedErr != nil {
			checkErr = defferedErr
		} else {
			checkErr = checkTCP(host, port, timeout)
		}
	case "udp":
		host, defferedErr := probeTargetHost("udp", target, 0, allowPrivate)
		if defferedErr != nil {
			checkErr = defferedErr
		} else {
			checkErr = checkUDP(host, port, timeout)
		}
	case "icmp", "ping":
		host, defferedErr := probeTargetHost("icmp", target, 0, allowPrivate)
		if defferedErr != nil {
			checkErr = defferedErr
		} else if host != "" {
			checkErr = checkICMP(target, timeout)
		}
	case "dns":
		host, defferedErr := probeTargetHost("dns", target, 0, allowPrivate)
		if defferedErr != nil {
			checkErr = defferedErr
		} else if host != "" {
			checkErr = checkDNS(target, timeout)
		}
	case "tls":
		if _, defferedErr := probeTargetHost("tls", target, 443, allowPrivate); defferedErr != nil {
			checkErr = defferedErr
		} else {
			checkErr = checkTLS(target, port, timeout)
		}
	default:
		checkErr = fmt.Errorf("unknown type: %s", typ)
	}

	elapsed := int(time.Since(start).Milliseconds())
	status := "up"
	if checkErr != nil {
		var netErr net.Error
		if errors.As(checkErr, &netErr) && netErr.Timeout() {
			status = "slow" // yellow – connected but timed out waiting for response
		} else {
			status = "down" // red – cannot connect
		}
	}

	s.recordResult(id, "panel", "", status, elapsed, checkErr, certInfo, "", nil)
}

func (s *ServiceMonitorService) recordResult(id uint, probeType, probeID, status string, elapsed int, checkErr error, certInfo *certCheckInfo, errorCode string, metadata map[string]any) {
	now := time.Now()
	s.saveProbeResult(id, probeType, probeID, status, elapsed, checkErr, now, errorCode, metadata)
	s.commitMonitorStatus(id, status, elapsed, checkErr, now, certInfo, metadata)
}

func (s *ServiceMonitorService) saveProbeResult(id uint, probeType, probeID, status string, elapsed int, checkErr error, checkedAt time.Time, errorCode string, metadata map[string]any) {
	errorText := ""
	if checkErr != nil {
		errorText = checkErr.Error()
	}
	probeName, probeLocation := resolveProbeMetadata(probeType, probeID)
	resultRepo := repositories.NewServiceMonitorResultRepository()
	if saveErr := resultRepo.Save(&models.ServiceMonitorResult{
		MonitorID:     id,
		ProbeType:     normalizeProbeType(probeType),
		ProbeID:       probeID,
		ProbeName:     probeName,
		ProbeLocation: probeLocation,
		Status:        status,
		ResponseTime:  elapsed,
		Error:         errorText,
		ErrorCode:     errorCode,
		Metadata:      metadata,
		CheckedAt:     checkedAt,
		CreatedAt:     checkedAt,
		UpdatedAt:     checkedAt,
	}); saveErr == nil {
		go resultRepo.PruneBefore(id, checkedAt.Add(-serviceMonitorResultRetention))
	} else {
		facades.Log().Warningf("保存服务监测原始结果失败: monitor_id=%d, error=%v", id, saveErr)
	}
}

// certCheckInfo 面板 HTTPS 证书有效期检测结果
type certCheckInfo struct {
	ExpiresAt time.Time
	DaysLeft  int
}

func (s *ServiceMonitorService) commitMonitorStatus(id uint, status string, elapsed int, checkErr error, now time.Time, certInfo *certCheckInfo, metadata map[string]any) {
	if status == "unknown" {
		s.commitUnknownMonitorStatus(id, elapsed, checkErr, now)
		return
	}

	unlock := s.lockMonitor(id)
	defer unlock()

	repo := repositories.GetServiceMonitorRepository()
	previous, _ := repo.GetByID(id)
	oldStatus := "unknown"
	if previous != nil && previous.Status != "" {
		oldStatus = previous.Status
	}
	effectiveStatus, consecutiveFailures, consecutiveSuccesses := applyMonitorStability(previous, oldStatus, status)

	update := map[string]interface{}{
		"status":                effectiveStatus,
		"response_time":         elapsed,
		"last_check_at":         now,
		"consecutive_failures":  consecutiveFailures,
		"consecutive_successes": consecutiveSuccesses,
	}
	if certInfo != nil {
		update["cert_expires_at"] = certInfo.ExpiresAt
		update["cert_days_left"] = certInfo.DaysLeft
	}
	if len(metadata) > 0 {
		// 映射更新不会走模型字段的 serializer。直接写 map 会让 SQLite
		// 报 unsupported type，进而连同 status 一起无法持久化。
		encodedMetadata, err := json.Marshal(metadata)
		if err != nil {
			facades.Log().Warningf("序列化服务监测元数据失败: monitor_id=%d, error=%v", id, err)
			return
		}
		update["last_metadata"] = string(encodedMetadata)
		update["metadata_checked_at"] = now
	} else if isProtocolMonitor(previous) {
		update["last_metadata"] = nil
		update["metadata_checked_at"] = nil
	}
	if err := repo.Update(id, update); err != nil {
		facades.Log().Warningf("更新服务监测状态失败: monitor_id=%d, error=%v", id, err)
		return
	}

	// 保存历史记录，按时间保留，列表接口只读取最近若干条用于展示。
	histRepo := repositories.GetServiceMonitorHistoryRepository()
	histEntry := &models.ServiceMonitorHistory{
		MonitorID:    id,
		Status:       effectiveStatus,
		ResponseTime: elapsed,
		CheckedAt:    now,
	}
	if saveErr := histRepo.Save(histEntry); saveErr == nil {
		go histRepo.PruneBefore(id, now.Add(-serviceMonitorHistoryRetention))
	}

	GetWebSocketService().GetManager().BroadcastToFrontend(map[string]interface{}{
		"type": "service_monitor_update",
		"data": map[string]interface{}{
			"id":                  id,
			"status":              effectiveStatus,
			"response_time":       elapsed,
			"last_check_at":       now.Format(time.RFC3339),
			"last_metadata":       metadata,
			"metadata_checked_at": now.Format(time.RFC3339),
			"history_entry": map[string]interface{}{
				"status":        effectiveStatus,
				"response_time": elapsed,
				"checked_at":    now.Format(time.RFC3339),
			},
		},
	})

	if oldStatus != effectiveStatus {
		if effectiveStatus == "up" && oldStatus != "unknown" {
			NewIncidentService().ResolveServiceMonitorIncident(id, oldStatus, elapsed)
			NewAlertService().NotifyServiceMonitorRecovery(id, oldStatus, elapsed)
		} else if effectiveStatus != "up" {
			NewIncidentService().OpenOrUpdateServiceMonitorIncident(id, effectiveStatus, elapsed, checkErr)
			NewAlertService().NotifyServiceMonitorProblem(id, effectiveStatus, elapsed, checkErr)
		}
	}

	if checkErr != nil {
		facades.Log().Warningf("服务监测 [%d] 检测失败: %v", id, checkErr)
	}
}

func isProtocolMonitor(monitor *models.ServiceMonitor) bool {
	return monitor != nil && isProtocolMonitorType(monitor.Type)
}

// commitUnknownMonitorStatus records that no monitoring point produced a
// trustworthy result. It deliberately skips incidents and alerts because probe
// availability is not evidence that the monitored target is unavailable.
func (s *ServiceMonitorService) commitUnknownMonitorStatus(id uint, elapsed int, checkErr error, now time.Time) {
	unlock := s.lockMonitor(id)
	defer unlock()

	repo := repositories.GetServiceMonitorRepository()
	_ = repo.Update(id, map[string]interface{}{
		"status":              "unknown",
		"response_time":       elapsed,
		"last_check_at":       now,
		"last_metadata":       nil,
		"metadata_checked_at": nil,
	})

	histRepo := repositories.GetServiceMonitorHistoryRepository()
	if err := histRepo.Save(&models.ServiceMonitorHistory{
		MonitorID:    id,
		Status:       "unknown",
		ResponseTime: elapsed,
		CheckedAt:    now,
	}); err == nil {
		go histRepo.PruneBefore(id, now.Add(-serviceMonitorHistoryRetention))
	}

	GetWebSocketService().GetManager().BroadcastToFrontend(map[string]interface{}{
		"type": "service_monitor_update",
		"data": map[string]interface{}{
			"id":            id,
			"status":        "unknown",
			"response_time": elapsed,
			"last_check_at": now.Format(time.RFC3339),
			"history_entry": map[string]interface{}{
				"status":        "unknown",
				"response_time": elapsed,
				"checked_at":    now.Format(time.RFC3339),
			},
		},
	})
	if checkErr != nil {
		facades.Log().Warningf("服务监测 [%d] 监测点不可用: %v", id, checkErr)
	}
}

func applyMonitorStability(previous *models.ServiceMonitor, oldStatus, nextStatus string) (string, int, int) {
	failureThreshold := 1
	recoveryThreshold := 1
	failures := 0
	successes := 0
	if previous != nil {
		if previous.FailureThreshold > 0 {
			failureThreshold = previous.FailureThreshold
		}
		if previous.RecoveryThreshold > 0 {
			recoveryThreshold = previous.RecoveryThreshold
		}
		failures = previous.ConsecutiveFailures
		successes = previous.ConsecutiveSuccesses
	}

	if nextStatus == "up" {
		successes++
		failures = 0
		if oldStatus != "up" && oldStatus != "unknown" && successes < recoveryThreshold {
			return oldStatus, failures, successes
		}
		return "up", failures, successes
	}

	failures++
	successes = 0
	if (oldStatus == "up" || oldStatus == "unknown") && failures < failureThreshold {
		return oldStatus, failures, successes
	}
	return nextStatus, failures, successes
}

// 处理Agent结果。Agent 只能为其被指派的监测点上报结果，且状态值必须在
// up/down/slow 白名单内；否则拒绝，防止越权伪造监测状态。
func (s *ServiceMonitorService) HandleAgentResult(data map[string]interface{}, serverID string) {
	monitorID, _ := data["monitor_id"].(float64)
	status, _ := data["status"].(string)
	responseTime, _ := data["response_time"].(float64)
	if monitorID == 0 || status == "" {
		return
	}
	switch status {
	case "up", "down", "slow":
	default:
		facades.Log().Warningf("拒绝非法监测状态上报: monitor_id=%d, status=%q", uint(monitorID), status)
		return
	}
	id := uint(monitorID)
	if serverID == "" {
		facades.Log().Warningf("拒绝未带身份的监测结果上报: monitor_id=%d", id)
		return
	}
	if !s.agentAssignedToMonitor(id, serverID) {
		facades.Log().Warningf("拒绝越权监测结果上报: monitor_id=%d, server_id=%s", id, serverID)
		return
	}
	checkID, _ := data["check_id"].(string)
	elapsed := int(responseTime)
	errText, _ := data["error"].(string)
	if len(errText) > maxAgentResultErrorLength {
		errText = errText[:maxAgentResultErrorLength]
	}
	var checkErr error
	if strings.TrimSpace(errText) != "" {
		checkErr = errors.New(errText)
	}
	if checkID != "" {
		s.addPendingAgentResult(id, checkID, serverID, status, elapsed, checkErr)
		return
	}
	s.recordResult(id, "agent", serverID, status, elapsed, checkErr, nil, "", nil)
}

// agentAssignedToMonitor 校验 serverID 是否在该监测点的探测点列表中。
func (s *ServiceMonitorService) agentAssignedToMonitor(monitorID uint, serverID string) bool {
	monitor, err := repositories.GetServiceMonitorRepository().GetByID(monitorID)
	if err != nil || monitor == nil {
		return false
	}
	for _, sid := range monitor.ServerIDs {
		if sid == serverID {
			return true
		}
	}
	return false
}

const maxAgentResultErrorLength = 4096

func (s *ServiceMonitorService) createPendingCheck(monitorID uint, checkID, commandID string, serverIDs []string, timeoutSec int) {
	s.rounds.Open(monitorID, checkID, commandID, serverIDs, timeoutSec, func() {
		s.finalizePendingCheck(monitorID, checkID, true)
	})
}

func (s *ServiceMonitorService) addPendingAgentResult(monitorID uint, checkID, serverID, status string, elapsed int, checkErr error) {
	if serverID == "" {
		serverID = "unknown"
	}
	now := time.Now()
	s.saveProbeResult(monitorID, "agent", serverID, status, elapsed, checkErr, now, "", nil)

	if s.rounds.AddResult(monitorID, checkID, probeCheckResult{
		probeID:      serverID,
		status:       status,
		responseTime: elapsed,
		err:          checkErr,
	}) {
		s.finalizePendingCheck(monitorID, checkID, false)
	}
}

func (s *ServiceMonitorService) finalizePendingCheck(monitorID uint, checkID string, markMissing bool) {
	pending := s.rounds.Complete(monitorID, checkID)
	if pending == nil {
		return
	}

	results := make([]probeCheckResult, 0, len(pending.expected))
	for _, result := range pending.results {
		results = append(results, result)
	}
	if markMissing {
		manager := GetWebSocketService().GetManager()
		for serverID := range pending.expected {
			if _, ok := pending.results[serverID]; ok {
				continue
			}
			err := errors.New("monitoring point unavailable: agent offline")
			if conn, online := manager.GetAgentConnection(serverID); online && !conn.IsClosed() {
				err = errors.New("monitoring point unavailable: agent result timeout")
				cancelData := map[string]interface{}{
					"monitor_id": monitorID,
					"check_id":   checkID,
				}
				cancelPayload := map[string]interface{}{
					"command":    "service_check_cancel",
					"command_id": uuid.NewString(),
					"data":       cancelData,
				}
				if sig, ts, sigErr := SignAgentCommand(cancelData, cancelPayload["command_id"].(string)); sigErr == nil {
					cancelPayload["sig"] = sig
					cancelPayload["sig_ts"] = ts
					_ = manager.SendToAgent(serverID, cancelPayload)
				} else {
					// 取消命令签名失败时跳过发送：Agent 端会拒绝未签名命令，
					// 发送只是徒劳；迟到的结果由 Agent 侧 deadline 兜底丢弃
					facades.Log().Errorf("服务监测取消命令签名失败: monitor_id=%d, error=%v", monitorID, sigErr)
				}
			}
			_ = NewAgentTaskService().CancelByCommandID(serverID, pending.commandID, "监测轮次已截止")
			s.saveProbeResult(monitorID, "agent", serverID, "unknown", 0, err, time.Now(), "probe_unavailable", nil)
		}
	}

	if !s.rounds.IsLatest(monitorID, checkID) {
		return
	}

	if len(results) == 0 {
		err := errors.New("all monitoring points unavailable")
		s.commitMonitorStatus(monitorID, "unknown", 0, err, time.Now(), nil, nil)
		return
	}

	now := time.Now()
	status, elapsed, err := aggregateProbeResults(results)
	s.commitMonitorStatus(monitorID, status, elapsed, err, now, nil, nil)
}

func aggregateProbeResults(results []probeCheckResult) (string, int, error) {
	upCount := 0
	slowCount := 0
	downCount := 0
	maxElapsed := 0
	errMessages := make([]string, 0)

	for _, result := range results {
		if result.responseTime > maxElapsed {
			maxElapsed = result.responseTime
		}
		switch result.status {
		case "up":
			upCount++
		case "slow":
			slowCount++
		default:
			downCount++
			if result.err != nil {
				errMessages = append(errMessages, fmt.Sprintf("%s: %v", result.probeID, result.err))
			}
		}
	}

	if downCount == 0 && slowCount == 0 && upCount > 0 {
		return "up", maxElapsed, nil
	}
	if upCount == 0 && slowCount == 0 && downCount > 0 {
		return "down", maxElapsed, joinAggregateErrors(errMessages, "all probes failed")
	}

	message := fmt.Sprintf("partial probe failure: up=%d slow=%d down=%d", upCount, slowCount, downCount)
	if len(errMessages) > 0 {
		message += "; " + strings.Join(errMessages, "; ")
	}
	return "slow", maxElapsed, errors.New(message)
}

func joinAggregateErrors(messages []string, fallback string) error {
	if len(messages) == 0 {
		return errors.New(fallback)
	}
	return errors.New(strings.Join(messages, "; "))
}

func normalizeProbeType(probeType string) string {
	if probeType == "agent" || probeType == "panel" {
		return probeType
	}
	return "panel"
}

func resolveProbeMetadata(probeType, probeID string) (string, string) {
	if normalizeProbeType(probeType) == "panel" {
		return "Panel", "本地面板"
	}
	if probeID == "" || strings.Contains(probeID, ",") || probeID == "unknown" {
		return probeID, ""
	}
	server, err := repositories.NewServerRepository().GetByID(probeID)
	if err != nil || server == nil {
		return probeID, ""
	}
	name := strings.TrimSpace(server.Name)
	if name == "" {
		name = strings.TrimSpace(server.Hostname)
	}
	if name == "" {
		name = probeID
	}
	return name, strings.TrimSpace(server.Location)
}

// monitorAllowPrivateTargetsSetting 控制服务监测是否允许探测内网/保留地址。
// 默认禁止：面板直连探测是受限 SSRF 面，管理员账号被盗或 CSRF 失效时
// 可被用来探测内网。用户可在系统设置中显式开启。
const monitorAllowPrivateTargetsSetting = "monitor_allow_private_targets"

// monitorAllowPrivateTargetsOverride 仅供单元测试覆盖（httptest 监听 loopback）。
var monitorAllowPrivateTargetsOverride *bool

func monitorAllowsPrivateTargets() (allowed bool) {
	if monitorAllowPrivateTargetsOverride != nil {
		return *monitorAllowPrivateTargetsOverride
	}
	// 设置读取依赖 ORM；在无数据库的单元测试环境可能 panic，此时按默认（禁止）处理。
	defer func() {
		if r := recover(); r != nil {
			allowed = false
		}
	}()
	return repositories.NewSystemSettingRepository().GetBool(monitorAllowPrivateTargetsSetting, false)
}
