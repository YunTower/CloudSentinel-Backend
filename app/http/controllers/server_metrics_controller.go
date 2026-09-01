package controllers

// 服务器历史性能指标查询端点（CPU / 内存 / 磁盘IO / 网络）。
// 从 server_controller.go 拆出：该文件此前同时承载 CRUD、告警、
// WS 联动与指标聚合查询，职责过多；指标查询自成一块，便于独立维护。

import (
	"fmt"
	"strconv"
	"time"

	"github.com/goravel/framework/contracts/http"
	"goravel/app/facades"
	"goravel/app/services"
	"goravel/app/utils"
)

// GetServerMetricsCPU 获取服务器CPU负载历史数据
func (c *ServerController) GetServerMetricsCPU(ctx http.Context) http.Response {
	return c.getServerMetricsByType(ctx, "cpu")
}

// GetServerMetricsMemory 获取服务器内存负载历史数据
func (c *ServerController) GetServerMetricsMemory(ctx http.Context) http.Response {
	return c.getServerMetricsByType(ctx, "memory")
}

// GetServerMetricsDisk 获取服务器磁盘读写负载历史数据
func (c *ServerController) GetServerMetricsDisk(ctx http.Context) http.Response {
	return c.getServerMetricsByType(ctx, "disk")
}

// GetServerMetricsNetwork 获取服务器网络IO负载历史数据
func (c *ServerController) GetServerMetricsNetwork(ctx http.Context) http.Response {
	return c.getServerMetricsByType(ctx, "network")
}

// getServerMetricsByType 根据类型获取服务器历史性能指标
func (c *ServerController) getServerMetricsByType(ctx http.Context, metricType string) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "缺少服务器ID")
	}

	// 获取时间范围参数
	// 支持两种方式：start/end 日期时间参数，或 hours 参数
	var startTime time.Time
	var endTime time.Time = time.Now()

	// 优先使用 start/end 参数
	startParam := ctx.Request().Query("start", "")
	endParam := ctx.Request().Query("end", "")

	if startParam != "" {
		// 解析开始时间
		if timestamp, err := strconv.ParseInt(startParam, 10, 64); err == nil {
			startTime = time.Unix(timestamp, 0)
		} else {
			// 解析失败，尝试ISO格式
			parsedStart, err := time.Parse(time.RFC3339, startParam)
			if err != nil {
				parsedStart, err = time.Parse("2006-01-02T15:04:05Z07:00", startParam)
				if err != nil {
					parsedStart, err = time.Parse("2006-01-02 15:04:05", startParam)
				}
			}
			if err == nil {
				startTime = parsedStart
			} else {
				// 解析失败，使用默认值（最近24小时）
				startTime = time.Now().Add(-24 * time.Hour)
			}
		}
	} else {
		// 使用 hours 参数
		hours := 24
		if hoursParam := ctx.Request().Query("hours", ""); hoursParam != "" {
			var h int
			if _, err := fmt.Sscanf(hoursParam, "%d", &h); err == nil && h > 0 {
				hours = h
			}
		}
		// 限制最大时间范围为24小时
		if hours > 24 {
			hours = 24
		}
		startTime = time.Now().Add(-time.Duration(hours) * time.Hour)
	}

	if endParam != "" {
		// 解析结束时间
		if timestamp, err := strconv.ParseInt(endParam, 10, 64); err == nil {
			endTime = time.Unix(timestamp, 0)
		} else {
			// 解析失败，尝试ISO格式
			parsedEnd, err := time.Parse(time.RFC3339, endParam)
			if err != nil {
				parsedEnd, err = time.Parse("2006-01-02T15:04:05Z07:00", endParam)
				if err != nil {
					parsedEnd, err = time.Parse("2006-01-02 15:04:05", endParam)
				}
			}
			if err == nil {
				endTime = parsedEnd
			}
		}
	}

	var metrics []map[string]interface{}
	// 初始化metrics为空切片，避免"model value required"错误
	metrics = []map[string]interface{}{}

	var err error

	// 计算时间范围（分钟）
	durationMinutes := int(endTime.Sub(startTime).Minutes())
	if durationMinutes <= 0 {
		durationMinutes = 60 // 默认1小时
	}

	// 根据时间范围计算采样间隔（分钟）
	var sampleIntervalMinutes int
	if durationMinutes <= 60 {
		// 1小时：每2分钟一个点，约30个点
		sampleIntervalMinutes = 2
	} else if durationMinutes <= 180 {
		// 3小时：每5分钟一个点，约36个点
		sampleIntervalMinutes = 5
	} else if durationMinutes <= 360 {
		// 6小时：每10分钟一个点，约36个点
		sampleIntervalMinutes = 10
	} else if durationMinutes <= 720 {
		// 12小时：每30分钟一个点，约24个点
		sampleIntervalMinutes = 30
	} else {
		// 24小时：每30分钟一个点，约48个点
		sampleIntervalMinutes = 30
	}

	switch metricType {
	case "cpu":
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		sql := `SELECT 
			datetime(CAST((timestamp_unix / ?) * ? AS INTEGER), 'unixepoch') AS timestamp,
			AVG(cpu_usage) AS cpu_usage
		FROM (
			SELECT 
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END AS timestamp_unix,
				cpu_usage
			FROM server_metrics
			WHERE server_id = ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) >= ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) <= ?
		)
		GROUP BY timestamp_unix / ?
		ORDER BY timestamp ASC`
		args := []interface{}{sampleIntervalSeconds, sampleIntervalSeconds, serverID, startTime.Unix()}
		if endParam != "" {
			args = append(args, endTime.Unix())
		} else {
			args = append(args, time.Now().Unix())
		}
		args = append(args, sampleIntervalSeconds)

		err = facades.Orm().Query().Raw(sql, args...).Scan(&metrics)

	case "memory":
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		sql := `SELECT 
			datetime(CAST((timestamp_unix / ?) * ? AS INTEGER), 'unixepoch') AS timestamp,
			AVG(memory_usage_percent) AS memory_usage
		FROM (
			SELECT 
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END AS timestamp_unix,
				memory_usage_percent
			FROM server_memory_history
			WHERE server_id = ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) >= ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) <= ?
		)
		GROUP BY timestamp_unix / ?
		ORDER BY timestamp ASC`
		args := []interface{}{sampleIntervalSeconds, sampleIntervalSeconds, serverID, startTime.Unix()}
		if endParam != "" {
			args = append(args, endTime.Unix())
		} else {
			args = append(args, time.Now().Unix())
		}
		args = append(args, sampleIntervalSeconds)

		err = facades.Orm().Query().Raw(sql, args...).Scan(&metrics)

	case "disk":
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		sql := `SELECT 
			datetime(CAST((timestamp_unix / ?) * ? AS INTEGER), 'unixepoch') AS timestamp,
			AVG(read_speed) AS disk_read,
			AVG(write_speed) AS disk_write
		FROM (
			SELECT 
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END AS timestamp_unix,
				read_speed,
				write_speed
			FROM server_disk_io
			WHERE server_id = ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) >= ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) <= ?
		)
		GROUP BY timestamp_unix / ?
		ORDER BY timestamp ASC`
		args := []interface{}{sampleIntervalSeconds, sampleIntervalSeconds, serverID, startTime.Unix()}
		if endParam != "" {
			args = append(args, endTime.Unix())
		} else {
			args = append(args, time.Now().Unix())
		}
		args = append(args, sampleIntervalSeconds)

		err = facades.Orm().Query().Raw(sql, args...).Scan(&metrics)

	case "network":
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		sql := `SELECT 
			datetime(CAST((timestamp_unix / ?) * ? AS INTEGER), 'unixepoch') AS timestamp,
			AVG(upload_speed) AS network_upload,
			AVG(download_speed) AS network_download
		FROM (
			SELECT 
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END AS timestamp_unix,
				upload_speed,
				download_speed
			FROM server_network_speed
			WHERE server_id = ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) >= ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) <= ?
		)
		GROUP BY timestamp_unix / ?
		ORDER BY timestamp ASC`
		args := []interface{}{sampleIntervalSeconds, sampleIntervalSeconds, serverID, startTime.Unix()}
		if endParam != "" {
			args = append(args, endTime.Unix())
		} else {
			args = append(args, time.Now().Unix())
		}
		args = append(args, sampleIntervalSeconds)

		err = facades.Orm().Query().Raw(sql, args...).Scan(&metrics)

	default:
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "无效的指标类型")
	}

	// 如果查询失败，记录警告但返回包含0值的数据点（不返回错误）
	if err != nil {
		facades.Log().Warningf("获取服务器%s指标失败（可能没有数据）: server_id=%s, error=%v", metricType, serverID, err)
		metrics = []map[string]interface{}{}
	}

	// 记录查询结果条数；缺失桶会在后续按采样间隔统一补齐（保证不返回空、且整段有覆盖）
	if len(metrics) == 0 {
		facades.Log().Infof("获取服务器%s指标无数据（或未覆盖完整桶），将补齐时间桶: server_id=%s, 时间范围=%d分钟, 采样间隔=%d分钟, startTime=%v, endTime=%v",
			metricType, serverID, durationMinutes, sampleIntervalMinutes, startTime, endTime)
	} else {
		// 查询成功且可能存在部分桶缺失
		facades.Log().Infof("获取服务器%s指标成功（可能缺失部分桶）: server_id=%s, 时间范围=%d分钟, 采样间隔=%d分钟, 数据量=%d, startTime=%v, endTime=%v",
			metricType, serverID, durationMinutes, sampleIntervalMinutes, len(metrics), startTime, endTime)
	}

	// 转换timestamp为Unix时间戳（秒），并处理数值字段保留两位小数（不四舍五入）
	for i := range metrics {
		if ts, ok := metrics[i]["timestamp"]; ok {
			var unixTimestamp int64
			switch v := ts.(type) {
			case time.Time:
				unixTimestamp = v.Unix()
			case string:
				// 尝试解析时间字符串
				if parsedTime, err := time.Parse(time.RFC3339, v); err == nil {
					unixTimestamp = parsedTime.Unix()
				} else if parsedTime, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
					unixTimestamp = parsedTime.Unix()
				} else {
					unixTimestamp = time.Now().Unix()
				}
			case int64:
				unixTimestamp = v
			case int:
				unixTimestamp = int64(v)
			case float64:
				unixTimestamp = int64(v)
			default:
				unixTimestamp = time.Now().Unix()
			}
			metrics[i]["timestamp"] = unixTimestamp
		}

		for key, value := range metrics[i] {
			if key == "timestamp" {
				continue // 跳过timestamp字段
			}
			// 使用统一的格式化函数
			metrics[i][key] = services.FormatMetricValue(value)
		}
	}

	// 补齐缺失时间桶：按采样间隔生成完整序列，缺失的桶用 0 填充。
	// 注意：SQL 的 group by 会把时间戳对齐到 sampleIntervalSeconds 的桶边界，
	// 因此补齐时也使用同样的 floor 对齐方式生成桶起点。
	sampleIntervalSeconds := int64(sampleIntervalMinutes * 60)
	startUnix := startTime.Unix()
	endUnix := endTime.Unix()
	bucketStart := (startUnix / sampleIntervalSeconds) * sampleIntervalSeconds

	metricsByTimestamp := make(map[int64]map[string]interface{}, len(metrics))
	for _, m := range metrics {
		if ts, ok := m["timestamp"]; ok {
			switch v := ts.(type) {
			case int64:
				metricsByTimestamp[v] = m
			case int:
				metricsByTimestamp[int64(v)] = m
			case float64:
				metricsByTimestamp[int64(v)] = m
			}
		}
	}

	filled := make([]map[string]interface{}, 0)
	for t := bucketStart; t <= endUnix; t += sampleIntervalSeconds {
		entry := map[string]interface{}{
			"timestamp": t,
		}

		if found, ok := metricsByTimestamp[t]; ok {
			// 命中已有桶
			switch metricType {
			case "cpu":
				if v, ok := found["cpu_usage"]; ok && v != nil {
					entry["cpu_usage"] = v
				} else {
					entry["cpu_usage"] = 0.0
				}
			case "memory":
				if v, ok := found["memory_usage"]; ok && v != nil {
					entry["memory_usage"] = v
				} else {
					entry["memory_usage"] = 0.0
				}
			case "disk":
				if v, ok := found["disk_read"]; ok && v != nil {
					entry["disk_read"] = v
				} else {
					entry["disk_read"] = 0.0
				}
				if v, ok := found["disk_write"]; ok && v != nil {
					entry["disk_write"] = v
				} else {
					entry["disk_write"] = 0.0
				}
			case "network":
				if v, ok := found["network_upload"]; ok && v != nil {
					entry["network_upload"] = v
				} else {
					entry["network_upload"] = 0.0
				}
				if v, ok := found["network_download"]; ok && v != nil {
					entry["network_download"] = v
				} else {
					entry["network_download"] = 0.0
				}
			}
		} else {
			// 未命中桶：补 0
			switch metricType {
			case "cpu":
				entry["cpu_usage"] = 0.0
			case "memory":
				entry["memory_usage"] = 0.0
			case "disk":
				entry["disk_read"] = 0.0
				entry["disk_write"] = 0.0
			case "network":
				entry["network_upload"] = 0.0
				entry["network_download"] = 0.0
			}
		}

		filled = append(filled, entry)
	}

	metrics = filled

	// 再走一次统一格式化，确保补齐出来的 0 值也符合数值展示规范。
	for i := range metrics {
		for key, value := range metrics[i] {
			if key == "timestamp" {
				continue
			}
			metrics[i][key] = services.FormatMetricValue(value)
		}
	}

	return utils.SuccessResponse(ctx, "获取成功", metrics)
}
