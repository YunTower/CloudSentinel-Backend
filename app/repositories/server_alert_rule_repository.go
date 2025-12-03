package repositories

import (
	"goravel/app/models"

	"github.com/goravel/framework/facades"
)

// ServerAlertRuleRepository 服务器告警规则
type ServerAlertRuleRepository struct{}

// NewServerAlertRuleRepository 创建服务器告警规则实例
func NewServerAlertRuleRepository() *ServerAlertRuleRepository {
	return &ServerAlertRuleRepository{}
}

// GetByServerIDAndType 根据服务器ID和规则类型获取规则
func (r *ServerAlertRuleRepository) GetByServerIDAndType(serverID *string, ruleType string) (*models.ServerAlertRule, error) {
	var rule models.ServerAlertRule
	query := facades.Orm().Query().Where("rule_type", ruleType)
	if serverID == nil {
		query = query.Where("server_id", nil)
	} else {
		query = query.Where("server_id", *serverID)
	}
	err := query.First(&rule)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetByServerID 获取指定服务器的所有规则
func (r *ServerAlertRuleRepository) GetByServerID(serverID string) ([]*models.ServerAlertRule, error) {
	var rules []*models.ServerAlertRule
	err := facades.Orm().Query().Where("server_id", serverID).Get(&rules)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// GetGlobalRules 获取所有全局规则（server_id 为 NULL）
func (r *ServerAlertRuleRepository) GetGlobalRules() ([]*models.ServerAlertRule, error) {
	var rules []*models.ServerAlertRule
	err := facades.Orm().Query().Where("server_id", nil).Get(&rules)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// CreateOrUpdate 创建或更新规则
func (r *ServerAlertRuleRepository) CreateOrUpdate(rule *models.ServerAlertRule) error {
	var existing models.ServerAlertRule
	query := facades.Orm().Query().Where("rule_type", rule.RuleType)
	if rule.ServerID == nil {
		query = query.Where("server_id", nil)
	} else {
		query = query.Where("server_id", *rule.ServerID)
	}
	err := query.First(&existing)

	if err != nil {
		// 不存在则创建
		return facades.Orm().Query().Create(rule)
	}

	// 存在则更新
	existing.Config = rule.Config
	return facades.Orm().Query().Save(&existing)
}

// DeleteByServerID 删除指定服务器的所有规则
func (r *ServerAlertRuleRepository) DeleteByServerID(serverID string) error {
	_, err := facades.Orm().Query().Model(&models.ServerAlertRule{}).Where("server_id", serverID).Delete()
	return err
}

// DeleteByServerIDAndType 删除指定服务器和规则类型的规则
func (r *ServerAlertRuleRepository) DeleteByServerIDAndType(serverID *string, ruleType string) error {
	query := facades.Orm().Query().Model(&models.ServerAlertRule{}).Where("rule_type", ruleType)
	if serverID == nil {
		query = query.Where("server_id", nil)
	} else {
		query = query.Where("server_id", *serverID)
	}
	_, err := query.Delete()
	return err
}


