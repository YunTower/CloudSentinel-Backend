package services

import (
	"strconv"
	"time"

	"github.com/goravel/framework/facades"
)

// LoginLockoutService IP锁定服务
type LoginLockoutService struct{}

// NewLoginLockoutService 创建新的IP锁定服务实例
func NewLoginLockoutService() *LoginLockoutService {
	return &LoginLockoutService{}
}

// getLockoutConfig 获取锁定配置
func (s *LoginLockoutService) getLockoutConfig() (maxAttempts int, lockoutSeconds int64, err error) {
	var maxLoginAttempts string
	var lockoutDurationSeconds string

	if err := facades.DB().Table("system_settings").Where("setting_key", "max_login_attempts").Value("setting_value", &maxLoginAttempts); err != nil {
		maxLoginAttempts = "5" // 默认值
	}

	if err := facades.DB().Table("system_settings").Where("setting_key", "lockout_duration").Value("setting_value", &lockoutDurationSeconds); err != nil {
		lockoutDurationSeconds = "900" // 默认值 15分钟
	}

	maxAttempts, _ = strconv.Atoi(maxLoginAttempts)
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	lockoutSeconds, _ = strconv.ParseInt(lockoutDurationSeconds, 10, 64)
	if lockoutSeconds <= 0 {
		lockoutSeconds = 900 // 默认15分钟
	}

	return maxAttempts, lockoutSeconds, nil
}

// IsIPLocked 检查IP是否被锁定
func (s *LoginLockoutService) IsIPLocked(ip string) (bool, error) {
	lockKey := "login_lockout:" + ip
	locked := facades.Cache().Get(lockKey, "")
	if locked == nil || locked == "" {
		return false, nil
	}

	return true, nil
}

// GetFailedAttempts 获取IP的失败尝试次数
func (s *LoginLockoutService) GetFailedAttempts(ip string) (int, error) {
	attemptKey := "login_attempts:" + ip
	attempts := facades.Cache().Get(attemptKey, 0)

	if attempts == nil || attempts == 0 {
		return 0, nil
	}

	count, ok := attempts.(int)
	if !ok {
		// 尝试转换为字符串再解析
		if str, ok := attempts.(string); ok {
			count, _ = strconv.Atoi(str)
		} else {
			count = 0
		}
	}

	return count, nil
}

// IncrementFailedAttempts 增加失败尝试次数，如果达到阈值则锁定IP
func (s *LoginLockoutService) IncrementFailedAttempts(ip string) error {
	maxAttempts, lockoutSeconds, err := s.getLockoutConfig()
	if err != nil {
		return err
	}

	attemptKey := "login_attempts:" + ip
	currentAttempts, err := s.GetFailedAttempts(ip)
	if err != nil {
		currentAttempts = 0
	}

	newAttempts := currentAttempts + 1

	// 设置失败次数，过期时间为锁定时间的2倍，确保在锁定期间不会丢失计数
	if err := facades.Cache().Put(attemptKey, newAttempts, time.Duration(lockoutSeconds*2)*time.Second); err != nil {
		return err
	}

	// 如果达到最大尝试次数，锁定IP
	if newAttempts >= maxAttempts {
		lockKey := "login_lockout:" + ip
		if err := facades.Cache().Put(lockKey, true, time.Duration(lockoutSeconds)*time.Second); err != nil {
			return err
		}
	}

	return nil
}

// ClearFailedAttempts 清除失败尝试次数
func (s *LoginLockoutService) ClearFailedAttempts(ip string) error {
	attemptKey := "login_attempts:" + ip
	facades.Cache().Forget(attemptKey)
	return nil
}
