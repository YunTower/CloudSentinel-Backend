package repositories

import (
	"goravel/app/models"
	"time"

	"github.com/goravel/framework/facades"
)

type AgentTaskRepository struct{}

func NewAgentTaskRepository() *AgentTaskRepository {
	return &AgentTaskRepository{}
}

func (r *AgentTaskRepository) Create(task *models.AgentTask) error {
	return facades.Orm().Query().Create(task)
}

func (r *AgentTaskRepository) PullPending(serverID string, limit int) ([]*models.AgentTask, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	now := time.Now()
	var tasks []*models.AgentTask
	err := facades.Orm().Query().Raw(
		`SELECT * FROM agent_tasks
		WHERE server_id = ? AND status = ? AND (available_at IS NULL OR available_at <= ?)
		ORDER BY created_at ASC
		LIMIT ?`,
		serverID, "pending", now, limit,
	).Scan(&tasks)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		_ = r.MarkDelivered(task.ID, now)
		task.Status = "delivered"
		task.DeliveredAt = &now
		task.Attempts++
	}
	return tasks, nil
}

func (r *AgentTaskRepository) MarkDelivered(id string, deliveredAt time.Time) error {
	_, err := facades.Orm().Query().Exec(
		`UPDATE agent_tasks
		SET status = ?, delivered_at = ?, attempts = attempts + 1, updated_at = ?
		WHERE id = ?`,
		"delivered", deliveredAt, deliveredAt, id,
	)
	return err
}

func (r *AgentTaskRepository) Complete(id, status, errText string) error {
	now := time.Now()
	update := map[string]interface{}{
		"status":       status,
		"completed_at": now,
		"updated_at":   now,
	}
	if errText != "" {
		update["error"] = errText
	}
	_, err := facades.Orm().Query().
		Model(&models.AgentTask{}).
		Where("id", id).
		Update(update)
	return err
}
