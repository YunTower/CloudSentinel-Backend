package migrations

import (
	"encoding/json"
	"goravel/app/repositories"

	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250120000001MergeConfigsToSystemSettings struct {
}

// Signature The unique signature for the migration.
func (r *M20250120000001MergeConfigsToSystemSettings) Signature() string {
	return "20250120000001_merge_configs_to_system_settings"
}

// Up Run the migrations.
func (r *M20250120000001MergeConfigsToSystemSettings) Up() error {
	settingRepo := repositories.GetSystemSettingRepository()

	// 1. 迁移 guest_access_config 到 system_settings
	if facades.Schema().HasTable("guest_access_config") {
		var guestConfigs []map[string]interface{}
		err := facades.Orm().Query().Table("guest_access_config").Get(&guestConfigs)
		if err == nil && len(guestConfigs) > 0 {
			config := guestConfigs[0]
			guestData := map[string]interface{}{
				"allow_guest":         config["allow_guest"],
				"enable_password":     config["enable_password"],
				"guest_password":      config["guest_password"],
				"hide_sensitive_info": config["hide_sensitive_info"],
			}
			jsonData, _ := json.Marshal(guestData)
			_ = settingRepo.SetValue("guest_access_config", string(jsonData))
		}
	}

	// 2. 迁移 permission_settings 到 system_settings
	if facades.Schema().HasTable("permission_settings") {
		var permConfigs []map[string]interface{}
		err := facades.Orm().Query().Table("permission_settings").Get(&permConfigs)
		if err == nil && len(permConfigs) > 0 {
			config := permConfigs[0]
			permData := map[string]interface{}{
				"session_timeout":    config["session_timeout"],
				"max_login_attempts": config["max_login_attempts"],
				"lockout_duration":   config["lockout_duration"],
				"jwt_secret":         config["jwt_secret"],
				"jwt_expiration":     config["jwt_expiration"],
			}
			jsonData, _ := json.Marshal(permData)
			_ = settingRepo.SetValue("permission_settings", string(jsonData))
		}
	}

	// 3. 迁移 monitor_config 到 system_settings
	if facades.Schema().HasTable("monitor_config") {
		var monitorConfigs []map[string]interface{}
		err := facades.Orm().Query().Table("monitor_config").Get(&monitorConfigs)
		if err == nil && len(monitorConfigs) > 0 {
			config := monitorConfigs[0]
			monitorData := map[string]interface{}{
				"refresh_interval":        config["refresh_interval"],
				"chart_data_points":       config["chart_data_points"],
				"enable_real_time_update": config["enable_real_time_update"],
				"cpu_threshold":           config["cpu_threshold"],
				"memory_threshold":        config["memory_threshold"],
				"disk_threshold":          config["disk_threshold"],
			}
			jsonData, _ := json.Marshal(monitorData)
			_ = settingRepo.SetValue("monitor_config", string(jsonData))
		}
	}

	// 4. 迁移 traffic_reset_config 到 system_settings
	if facades.Schema().HasTable("traffic_reset_config") {
		var trafficConfigs []map[string]interface{}
		err := facades.Orm().Query().Table("traffic_reset_config").Get(&trafficConfigs)
		if err == nil && len(trafficConfigs) > 0 {
			config := trafficConfigs[0]
			trafficData := map[string]interface{}{
				"reset_day":  config["reset_day"],
				"reset_hour": config["reset_hour"],
			}
			jsonData, _ := json.Marshal(trafficData)
			_ = settingRepo.SetValue("traffic_reset_config", string(jsonData))
		}
	}

	// 5. 迁移 log_cleanup_config 到 system_settings (多条记录，使用数组)
	if facades.Schema().HasTable("log_cleanup_config") {
		var cleanupConfigs []map[string]interface{}
		err := facades.Orm().Query().Table("log_cleanup_config").Get(&cleanupConfigs)
		if err == nil && len(cleanupConfigs) > 0 {
			// 转换为数组格式
			cleanupArray := make([]map[string]interface{}, 0, len(cleanupConfigs))
			for _, config := range cleanupConfigs {
				cleanupArray = append(cleanupArray, map[string]interface{}{
					"log_type":              config["log_type"],
					"cleanup_interval_days": config["cleanup_interval_days"],
					"keep_days":             config["keep_days"],
					"enabled":               config["enabled"],
					"last_cleanup_time":     config["last_cleanup_time"],
				})
			}
			jsonData, _ := json.Marshal(cleanupArray)
			_ = settingRepo.SetValue("log_cleanup_config", string(jsonData))
		}
	}

	// 6. 删除原配置表（如果存在）
	_ = facades.Schema().DropIfExists("guest_access_config")
	_ = facades.Schema().DropIfExists("permission_settings")
	_ = facades.Schema().DropIfExists("monitor_config")
	_ = facades.Schema().DropIfExists("traffic_reset_config")
	_ = facades.Schema().DropIfExists("log_cleanup_config")

	return nil
}

// Down Reverse the migrations.
func (r *M20250120000001MergeConfigsToSystemSettings) Down() error {
	// 回滚时重新创建表并恢复数据
	// 注意：这里只创建表结构，数据需要从备份恢复或重新插入

	// 重新创建 guest_access_config 表
	if !facades.Schema().HasTable("guest_access_config") {
		_ = facades.Schema().Create("guest_access_config", func(table schema.Blueprint) {
			table.ID()
			table.Boolean("allow_guest").Default(false)
			table.Boolean("enable_password").Default(false)
			table.String("guest_password", 255).Nullable()
			table.Boolean("hide_sensitive_info").Default(true)
			table.Timestamps()
		})
	}

	// 重新创建 permission_settings 表
	if !facades.Schema().HasTable("permission_settings") {
		_ = facades.Schema().Create("permission_settings", func(table schema.Blueprint) {
			table.ID()
			table.Integer("session_timeout").Default(3600)
			table.Integer("max_login_attempts").Default(5)
			table.Integer("lockout_duration").Default(900)
			table.String("jwt_secret", 255).Nullable()
			table.Integer("jwt_expiration").Default(86400)
			table.Timestamps()
		})
	}

	// 重新创建 monitor_config 表
	if !facades.Schema().HasTable("monitor_config") {
		_ = facades.Schema().Create("monitor_config", func(table schema.Blueprint) {
			table.ID()
			table.Integer("refresh_interval").Default(30)
			table.Integer("chart_data_points").Default(100)
			table.Boolean("enable_real_time_update").Default(true)
			table.Decimal("cpu_threshold").Default(80)
			table.Decimal("memory_threshold").Default(80)
			table.Decimal("disk_threshold").Default(80)
			table.Timestamps()
		})
	}

	// 重新创建 traffic_reset_config 表
	if !facades.Schema().HasTable("traffic_reset_config") {
		_ = facades.Schema().Create("traffic_reset_config", func(table schema.Blueprint) {
			table.ID()
			table.Integer("reset_day").Default(1)
			table.Integer("reset_hour").Default(0)
			table.Timestamps()
		})
	}

	// 重新创建 log_cleanup_config 表
	if !facades.Schema().HasTable("log_cleanup_config") {
		_ = facades.Schema().Create("log_cleanup_config", func(table schema.Blueprint) {
			table.ID()
			table.String("log_type", 50)
			table.Integer("cleanup_interval_days")
			table.Integer("keep_days")
			table.Boolean("enabled").Default(true)
			table.Timestamp("last_cleanup_time").Nullable()
			table.Timestamps()
		})
	}

	return nil
}
