package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"goravel/app/models"
	"goravel/app/repositories"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/goravel/framework/facades"
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
	mu            sync.Mutex
	stopChs       map[uint]chan struct{}
	pendingChecks map[string]*pendingServiceCheck
}

type probeCheckResult struct {
	probeID      string
	status       string
	responseTime int
	err          error
}

type pendingServiceCheck struct {
	monitorID uint
	checkID   string
	expected  map[string]struct{}
	results   map[string]probeCheckResult
	timer     *time.Timer
}

func GetServiceMonitorService() *ServiceMonitorService {
	serviceMonitorOnce.Do(func() {
		serviceMonitorInstance = &ServiceMonitorService{
			stopChs:       make(map[uint]chan struct{}),
			pendingChecks: make(map[string]*pendingServiceCheck),
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
	for key, pending := range s.pendingChecks {
		if pending.monitorID != id {
			continue
		}
		if pending.timer != nil {
			pending.timer.Stop()
		}
		delete(s.pendingChecks, key)
	}
}

func (s *ServiceMonitorService) runCheck(m *models.ServiceMonitor) {
	// 如果配置了 server_ids，则分发到 Agent；所有目标 Agent 都不可用时立即记录失败。
	if len(m.ServerIDs) > 0 {
		wsService := GetWebSocketService()
		manager := wsService.GetManager()
		checkID := uuid.NewString()
		commandID := uuid.NewString()
		s.createPendingCheck(m.ID, checkID, m.ServerIDs, m.Timeout)
		payload := map[string]interface{}{
			"command":    "service_check",
			"command_id": commandID,
			"data": map[string]interface{}{
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
			},
		}
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
	s.doCheck(m.ID, m.Type, m.Target, m.Port, m.Timeout, m.ExpectStatus, m.ExpectBody, m.HTTPMethod, m.HTTPHeaders, m.HTTPBody)
}

// 执行检查
func (s *ServiceMonitorService) doCheck(id uint, typ, target string, port, timeout, expectStatus int, expectBody, httpMethod, httpHeaders, httpBody string) {
	start := time.Now()
	var checkErr error

	if timeout <= 0 {
		timeout = 10
	}

	switch typ {
	case "http", "https":
		checkErr = checkHTTP(target, timeout, expectStatus, expectBody, httpMethod, httpHeaders, httpBody)
	case "tcp":
		checkErr = checkTCP(target, port, timeout)
	case "udp":
		checkErr = checkUDP(target, port, timeout)
	case "icmp", "ping":
		checkErr = checkICMP(target, timeout)
	case "dns":
		checkErr = checkDNS(target, timeout)
	case "tls":
		checkErr = checkTLS(target, port, timeout)
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

	s.recordResult(id, "panel", "", status, elapsed, checkErr)
}

func (s *ServiceMonitorService) recordResult(id uint, probeType, probeID, status string, elapsed int, checkErr error) {
	now := time.Now()
	s.saveProbeResult(id, probeType, probeID, status, elapsed, checkErr, now)
	s.commitMonitorStatus(id, status, elapsed, checkErr, now)
}

func (s *ServiceMonitorService) saveProbeResult(id uint, probeType, probeID, status string, elapsed int, checkErr error, checkedAt time.Time) {
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
		CheckedAt:     checkedAt,
		CreatedAt:     checkedAt,
		UpdatedAt:     checkedAt,
	}); saveErr == nil {
		go resultRepo.PruneBefore(id, checkedAt.Add(-serviceMonitorResultRetention))
	} else {
		facades.Log().Warningf("保存服务监测原始结果失败: monitor_id=%d, error=%v", id, saveErr)
	}
}

func (s *ServiceMonitorService) commitMonitorStatus(id uint, status string, elapsed int, checkErr error, now time.Time) {
	repo := repositories.GetServiceMonitorRepository()
	previous, _ := repo.GetByID(id)
	oldStatus := "unknown"
	if previous != nil && previous.Status != "" {
		oldStatus = previous.Status
	}
	effectiveStatus, consecutiveFailures, consecutiveSuccesses := applyMonitorStability(previous, oldStatus, status)

	_ = repo.Update(id, map[string]interface{}{
		"status":                effectiveStatus,
		"response_time":         elapsed,
		"last_check_at":         now,
		"consecutive_failures":  consecutiveFailures,
		"consecutive_successes": consecutiveSuccesses,
	})

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
			"id":            id,
			"status":        effectiveStatus,
			"response_time": elapsed,
			"last_check_at": now.Format(time.RFC3339),
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

// 处理Agent结果
func (s *ServiceMonitorService) HandleAgentResult(data map[string]interface{}, serverID string) {
	monitorID, _ := data["monitor_id"].(float64)
	status, _ := data["status"].(string)
	responseTime, _ := data["response_time"].(float64)
	if monitorID == 0 || status == "" {
		return
	}
	id := uint(monitorID)
	checkID, _ := data["check_id"].(string)
	elapsed := int(responseTime)
	errText, _ := data["error"].(string)
	var checkErr error
	if strings.TrimSpace(errText) != "" {
		checkErr = errors.New(errText)
	}
	if checkID != "" {
		s.addPendingAgentResult(id, checkID, serverID, status, elapsed, checkErr)
		return
	}
	s.recordResult(id, "agent", serverID, status, elapsed, checkErr)
}

func (s *ServiceMonitorService) createPendingCheck(monitorID uint, checkID string, serverIDs []string, timeoutSec int) {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	key := pendingCheckKey(monitorID, checkID)
	expected := make(map[string]struct{}, len(serverIDs))
	for _, serverID := range serverIDs {
		if serverID == "" {
			continue
		}
		expected[serverID] = struct{}{}
	}
	timer := time.AfterFunc(time.Duration(timeoutSec+2)*time.Second, func() {
		s.finalizePendingCheck(monitorID, checkID, true)
	})

	s.mu.Lock()
	if old, ok := s.pendingChecks[key]; ok && old.timer != nil {
		old.timer.Stop()
	}
	s.pendingChecks[key] = &pendingServiceCheck{
		monitorID: monitorID,
		checkID:   checkID,
		expected:  expected,
		results:   make(map[string]probeCheckResult, len(expected)),
		timer:     timer,
	}
	s.mu.Unlock()
}

func (s *ServiceMonitorService) addPendingAgentResult(monitorID uint, checkID, serverID, status string, elapsed int, checkErr error) {
	if serverID == "" {
		serverID = "unknown"
	}
	now := time.Now()
	s.saveProbeResult(monitorID, "agent", serverID, status, elapsed, checkErr, now)

	key := pendingCheckKey(monitorID, checkID)
	shouldFinalize := false

	s.mu.Lock()
	pending, ok := s.pendingChecks[key]
	if ok {
		pending.results[serverID] = probeCheckResult{
			probeID:      serverID,
			status:       status,
			responseTime: elapsed,
			err:          checkErr,
		}
		shouldFinalize = len(pending.expected) > 0 && len(pending.results) >= len(pending.expected)
	}
	s.mu.Unlock()

	if shouldFinalize {
		s.finalizePendingCheck(monitorID, checkID, false)
	}
}

func (s *ServiceMonitorService) finalizePendingCheck(monitorID uint, checkID string, markMissing bool) {
	key := pendingCheckKey(monitorID, checkID)

	s.mu.Lock()
	pending, ok := s.pendingChecks[key]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.pendingChecks, key)
	if pending.timer != nil {
		pending.timer.Stop()
	}
	results := make([]probeCheckResult, 0, len(pending.expected))
	for _, result := range pending.results {
		results = append(results, result)
	}
	if markMissing {
		for serverID := range pending.expected {
			if _, ok := pending.results[serverID]; ok {
				continue
			}
			err := errors.New("agent result timeout")
			result := probeCheckResult{probeID: serverID, status: "down", responseTime: 0, err: err}
			results = append(results, result)
		}
	}
	s.mu.Unlock()

	if len(results) == 0 {
		err := errors.New("no available agent for service check")
		s.recordResult(monitorID, "agent", strings.Join(mapKeys(pending.expected), ","), "down", 0, err)
		return
	}

	now := time.Now()
	if markMissing {
		for _, result := range results {
			if result.err != nil && result.err.Error() == "agent result timeout" {
				s.saveProbeResult(monitorID, "agent", result.probeID, result.status, result.responseTime, result.err, now)
			}
		}
	}
	status, elapsed, err := aggregateProbeResults(results)
	s.commitMonitorStatus(monitorID, status, elapsed, err, now)
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

func pendingCheckKey(monitorID uint, checkID string) string {
	return fmt.Sprintf("%d:%s", monitorID, checkID)
}

func mapKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
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

// 检查HTTP服务
func checkHTTP(target string, timeoutSec, expectStatus int, expectBody, method, headersJSON, requestBody string) error {
	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}
	method = normalizeRequestMethod(method)
	var body io.Reader
	if requestBody != "" && methodAllowsBody(method) {
		body = strings.NewReader(requestBody)
	}
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return err
	}
	headers, err := parseHTTPHeaders(headersJSON)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if requestBody != "" && methodAllowsBody(method) && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
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
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // 限制 1MB 防止内存耗尽
		if err != nil {
			return fmt.Errorf("读取响应体失败: %v", err)
		}
		if !strings.Contains(string(body), expectBody) {
			return fmt.Errorf("响应体不包含期望内容")
		}
	}
	return nil
}

func normalizeRequestMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return strings.ToUpper(strings.TrimSpace(method))
	default:
		return "GET"
	}
}

func methodAllowsBody(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func parseHTTPHeaders(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	headers := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, fmt.Errorf("http headers must be a JSON object: %w", err)
	}
	for key := range headers {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("http header name cannot be empty")
		}
	}
	return headers, nil
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

func checkICMP(target string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, _, err := splitMonitorTarget(target, 0)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	args := []string{"-c", "1", "-W", fmt.Sprintf("%d", timeoutSec), host}
	if runtime.GOOS == "windows" {
		args = []string{"-n", "1", "-w", fmt.Sprintf("%d", timeoutSec*1000), host}
	}
	cmd := exec.CommandContext(ctx, "ping", args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("ping timeout")
	}
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ping failed: %s", msg)
	}
	return nil
}

func checkDNS(target string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, _, err := splitMonitorTarget(target, 0)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return fmt.Errorf("dns lookup failed: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("dns lookup returned no records")
	}
	return nil
}

func checkTLS(target string, port int, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, resolvedPort, err := splitMonitorTarget(target, 443)
	if err != nil {
		return err
	}
	if port > 0 {
		resolvedPort = port
	}

	dialer := &net.Dialer{Timeout: time.Duration(timeoutSec) * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", host, resolvedPort), &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("tls connection failed: %w", err)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("tls certificate not found")
	}
	cert := state.PeerCertificates[0]
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("tls certificate not valid before %s", cert.NotBefore.Format(time.RFC3339))
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("tls certificate expired at %s", cert.NotAfter.Format(time.RFC3339))
	}
	return nil
}

func splitMonitorTarget(target string, defaultPort int) (string, int, error) {
	value := strings.TrimSpace(target)
	if value == "" {
		return "", 0, fmt.Errorf("target is empty")
	}

	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", 0, err
		}
		host := parsed.Hostname()
		if host == "" {
			return "", 0, fmt.Errorf("target host is empty")
		}
		port := defaultPort
		if parsed.Port() != "" {
			if _, err := fmt.Sscanf(parsed.Port(), "%d", &port); err != nil {
				return "", 0, fmt.Errorf("invalid target port")
			}
		}
		return host, port, nil
	}

	host := value
	port := defaultPort
	if h, p, err := net.SplitHostPort(value); err == nil {
		host = h
		if _, err := fmt.Sscanf(p, "%d", &port); err != nil {
			return "", 0, fmt.Errorf("invalid target port")
		}
	}
	if host == "" {
		return "", 0, fmt.Errorf("target host is empty")
	}
	return host, port, nil
}
