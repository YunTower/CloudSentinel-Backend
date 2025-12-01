package controllers

import (
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/utils"
	"strconv"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type ServerGroupController struct{}

func NewServerGroupController() *ServerGroupController {
	return &ServerGroupController{}
}

// GetGroups 获取所有分组
func (c *ServerGroupController) GetGroups(ctx http.Context) http.Response {
	groupRepo := repositories.GetServerGroupRepository()
	groups, err := groupRepo.GetAll()
	if err != nil {
		facades.Log().Errorf("获取分组列表失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "获取分组列表失败", err)
	}

	return utils.SuccessResponse(ctx, "获取成功", groups)
}

// CreateGroup 创建分组
func (c *ServerGroupController) CreateGroup(ctx http.Context) http.Response {
	type CreateGroupRequest struct {
		Name        string `json:"name" form:"name"`
		Description string `json:"description" form:"description"`
		Color       string `json:"color" form:"color"`
	}

	var req CreateGroupRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, http.StatusBadRequest, "请求参数错误", err)
	}

	// 验证必填字段
	if req.Name == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "分组名称为必填项")
	}

	group := &models.ServerGroup{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
	}

	groupRepo := repositories.GetServerGroupRepository()
	if err := groupRepo.Create(group); err != nil {
		facades.Log().Errorf("创建分组失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "创建分组失败", err)
	}

	facades.Log().Infof("成功创建分组: %s", req.Name)
	return utils.SuccessResponseWithStatus(ctx, http.StatusCreated, "分组创建成功", group)
}

// UpdateGroup 更新分组
func (c *ServerGroupController) UpdateGroup(ctx http.Context) http.Response {
	idStr := ctx.Request().Route("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "无效的分组ID")
	}

	type UpdateGroupRequest struct {
		Name        string `json:"name" form:"name"`
		Description string `json:"description" form:"description"`
		Color       string `json:"color" form:"color"`
	}

	var req UpdateGroupRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, http.StatusBadRequest, "请求参数错误", err)
	}

	groupRepo := repositories.GetServerGroupRepository()
	group, err := groupRepo.GetByID(uint(id))
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusNotFound, "分组不存在")
	}

	group.Name = req.Name
	group.Description = req.Description
	group.Color = req.Color

	if err := groupRepo.Update(group); err != nil {
		facades.Log().Errorf("更新分组失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "更新分组失败", err)
	}

	return utils.SuccessResponse(ctx, "更新成功", group)
}

// DeleteGroup 删除分组
func (c *ServerGroupController) DeleteGroup(ctx http.Context) http.Response {
	idStr := ctx.Request().Route("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "无效的分组ID")
	}

	groupRepo := repositories.GetServerGroupRepository()
	if err := groupRepo.Delete(uint(id)); err != nil {
		facades.Log().Errorf("删除分组失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "删除分组失败", err)
	}

	return utils.SuccessResponse(ctx, "删除成功", nil)
}
