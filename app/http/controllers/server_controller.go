package controllers

import (
	"time"

	"github.com/google/uuid"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type ServerController struct{}

func NewServerController() *ServerController {
	return &ServerController{}
}

// CreateServer 创建服务器
func (c *ServerController) CreateServer(ctx http.Context) http.Response {
	type CreateServerRequest struct {
		Name     string `json:"name" form:"name"`
		IP       string `json:"ip" form:"ip"`
		Port     int    `json:"port" form:"port"`
		Location string `json:"location" form:"location"`
		OS       string `json:"os" form:"os"`
	}

	var req CreateServerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
	}

	// 验证必填字段
	if req.Name == "" || req.IP == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "名称和IP地址为必填项",
		})
	}

	// 设置默认端口
	if req.Port == 0 {
		req.Port = 22
	}

	// 验证端口范围
	if req.Port < 1 || req.Port > 65535 {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "端口号必须在1-65535之间",
		})
	}

	// 生成UUID作为server_id
	serverID := uuid.New().String()
	
	// 生成agent_key
	agentKey := uuid.New().String()

	now := time.Now().Unix()

	// 插入数据库
	serverData := map[string]interface{}{
		"id":         serverID,
		"name":       req.Name,
		"ip":         req.IP,
		"port":       req.Port,
		"status":     "offline",
		"location":   req.Location,
		"os":         req.OS,
		"agent_key":  agentKey,
		"cores":      1,
		"created_at": now,
		"updated_at": now,
	}

	_, err := facades.Orm().Query().Exec(
		"INSERT INTO servers (id, name, ip, port, status, location, os, agent_key, cores, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		serverData["id"], serverData["name"], serverData["ip"], serverData["port"],
		serverData["status"], serverData["location"], serverData["os"],
		serverData["agent_key"], serverData["cores"], serverData["created_at"], serverData["updated_at"],
	)

	if err != nil {
		facades.Log().Errorf("创建服务器失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "创建服务器失败",
			"error":   err.Error(),
		})
	}

	facades.Log().Infof("成功创建服务器: %s (IP: %s)", req.Name, req.IP)

	// 返回服务器信息和agent_key
	return ctx.Response().Status(http.StatusCreated).Json(http.Json{
		"status":  true,
		"message": "服务器创建成功",
		"data": map[string]interface{}{
			"id":         serverID,
			"name":       req.Name,
			"ip":         req.IP,
			"port":       req.Port,
			"status":     "offline",
			"location":   req.Location,
			"os":         req.OS,
			"agent_key":  agentKey,
			"created_at": now,
			"updated_at": now,
		},
	})
}

// GetServers 获取服务器列表
func (c *ServerController) GetServers(ctx http.Context) http.Response {
	var servers []map[string]interface{}
	err := facades.Orm().Query().Table("servers").
		Select("id", "name", "ip", "port", "status", "location", "os", "architecture", "kernel", "hostname", "cores", "agent_version", "system_name", "boot_time", "last_report_time", "uptime_days", "created_at", "updated_at").
		OrderBy("created_at", "desc").
		Get(&servers)

	if err != nil {
		facades.Log().Errorf("获取服务器列表失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "获取服务器列表失败",
			"error":   err.Error(),
		})
	}

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "获取成功",
		"data":    servers,
	})
}

// UpdateServer 更新服务器信息
func (c *ServerController) UpdateServer(ctx http.Context) http.Response {
	serverID := ctx.Request().Input("id")
	if serverID == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "缺少服务器ID",
		})
	}

	type UpdateServerRequest struct {
		Name     string `json:"name" form:"name"`
		IP       string `json:"ip" form:"ip"`
		Port     int    `json:"port" form:"port"`
		Location string `json:"location" form:"location"`
		OS       string `json:"os" form:"os"`
	}

	var req UpdateServerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
	}

	// 构建更新数据
	updateData := make(map[string]interface{})
	if req.Name != "" {
		updateData["name"] = req.Name
	}
	if req.IP != "" {
		updateData["ip"] = req.IP
	}
	if req.Port > 0 {
		if req.Port < 1 || req.Port > 65535 {
			return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
				"status":  false,
				"message": "端口号必须在1-65535之间",
			})
		}
		updateData["port"] = req.Port
	}
	if req.Location != "" {
		updateData["location"] = req.Location
	}
	if req.OS != "" {
		updateData["os"] = req.OS
	}
	updateData["updated_at"] = time.Now().Unix()

	// 更新数据库
	_, err := facades.Orm().Query().Table("servers").
		Where("id", serverID).
		Update(updateData)

	if err != nil {
		facades.Log().Errorf("更新服务器失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "更新服务器失败",
			"error":   err.Error(),
		})
	}

	facades.Log().Infof("成功更新服务器: %s", serverID)

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "更新成功",
	})
}

// DeleteServer 删除服务器
func (c *ServerController) DeleteServer(ctx http.Context) http.Response {
	serverID := ctx.Request().Input("id")
	if serverID == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "缺少服务器ID",
		})
	}

	// 删除服务器（外键级联会自动删除相关数据）
	_, err := facades.Orm().Query().Table("servers").
		Where("id", serverID).
		Delete()

	if err != nil {
		facades.Log().Errorf("删除服务器失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "删除服务器失败",
			"error":   err.Error(),
		})
	}

	facades.Log().Infof("成功删除服务器: %s", serverID)

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "删除成功",
	})
}

