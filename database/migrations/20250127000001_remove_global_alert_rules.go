package migrations

import (
	"github.com/goravel/framework/facades"
)

type M20250127000001RemoveGlobalAlertRules struct {
}

// Signature The unique signature for the migration.
func (r *M20250127000001RemoveGlobalAlertRules) Signature() string {
	return "20250127000001_remove_global_alert_rules"
}

// Up Run the migrations.
func (r *M20250127000001RemoveGlobalAlertRules) Up() error {
	// 删除 server_alert_rules 表中 server_id 为 NULL 的记录（全局规则）
	if facades.Schema().HasTable("server_alert_rules") {
		_, err := facades.Orm().Query().Table("server_alert_rules").Where("server_id", nil).Delete()
		if err != nil {
			return err
		}
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250127000001RemoveGlobalAlertRules) Down() error {
	// 回滚时不做任何操作（已删除的全局规则无法恢复）
	return nil
}

