package repositories

import (
	"goravel/app/models"
	"time"

	"github.com/google/uuid"
	"github.com/goravel/framework/facades"
)

const agentTaskLeaseDuration = 10 * time.Minute

type AgentTaskRepository struct{}

func NewAgentTaskRepository() *AgentTaskRepository {
	return &AgentTaskRepository{}
}

func (r *AgentTaskRepository) Create(task *models.AgentTask) error {
	return facades.Orm().Query().Create(task)
}

// ClaimPending atomically leases work for one server. Expired leases and legacy
// delivered tasks are eligible for retry, while active leases cannot be claimed
// by concurrent pollers.
func (r *AgentTaskRepository) ClaimPending(serverID string, limit int) ([]*models.AgentTask, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	now := time.Now()
	leaseExpiresAt := now.Add(agentTaskLeaseDuration)
	leaseToken := uuid.NewString()

	// The conditional UPDATE is the claim operation. Selecting tasks before
	// updating would allow two concurrent Agent polls to execute the same task.
	result, err := facades.Orm().Query().Exec(
		`UPDATE agent_tasks
		SET status = ?, delivered_at = ?, lease_token = ?, lease_expires_at = ?,
			attempts = attempts + 1, updated_at = ?
		WHERE id IN (
			SELECT id FROM agent_tasks
			WHERE server_id = ?
				AND (available_at IS NULL OR available_at <= ?)
				AND (
					status = ?
					OR status = ?
					OR (status = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?))
				)
			ORDER BY created_at ASC
			LIMIT ?
		)`,
		"leased", now, leaseToken, leaseExpiresAt, now,
		serverID, now, "pending", "delivered", "leased", now, limit,
	)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected == 0 {
		return []*models.AgentTask{}, nil
	}

	var tasks []*models.AgentTask
	err = facades.Orm().Query().
		Where("server_id", serverID).
		Where("lease_token", leaseToken).
		OrderBy("created_at").
		Get(&tasks)
	return tasks, err
}

// Complete only accepts the active lease held by the same server. The boolean
// result distinguishes an invalid/expired/foreign lease from a database error.
func (r *AgentTaskRepository) Complete(serverID, id, leaseToken, status, errText string) (bool, error) {
	now := time.Now()
	update := map[string]interface{}{
		"status":           status,
		"completed_at":     now,
		"updated_at":       now,
		"lease_token":      nil,
		"lease_expires_at": nil,
	}
	if errText != "" {
		update["error"] = errText
	}
	result, err := facades.Orm().Query().
		Model(&models.AgentTask{}).
		Where("id", id).
		Where("server_id", serverID).
		Where("status", "leased").
		Where("lease_token", leaseToken).
		Where("lease_expires_at", ">", now).
		Update(update)
	if err != nil {
		return false, err
	}
	return result.RowsAffected == 1, nil
}

func (r *AgentTaskRepository) CancelByCommandID(serverID, commandID, reason string) error {
	now := time.Now()
	_, err := facades.Orm().Query().
		Model(&models.AgentTask{}).
		Where("server_id", serverID).
		Where("command_id", commandID).
		WhereIn("status", []any{"pending", "leased"}).
		Update(map[string]interface{}{
			"status":           "cancelled",
			"error":            reason,
			"completed_at":     now,
			"updated_at":       now,
			"lease_token":      nil,
			"lease_expires_at": nil,
		})
	return err
}
