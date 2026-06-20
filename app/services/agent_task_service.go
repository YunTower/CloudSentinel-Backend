package services

import (
	"goravel/app/models"
	"goravel/app/repositories"
	"time"

	"github.com/google/uuid"
)

type AgentTaskService struct {
	repo *repositories.AgentTaskRepository
}

func NewAgentTaskService() *AgentTaskService {
	return &AgentTaskService{repo: repositories.NewAgentTaskRepository()}
}

func (s *AgentTaskService) Enqueue(serverID, command, commandID string, payload map[string]interface{}) (*models.AgentTask, error) {
	if commandID == "" {
		commandID = uuid.NewString()
	}
	now := time.Now()
	task := &models.AgentTask{
		ID:          uuid.NewString(),
		ServerID:    serverID,
		Command:     command,
		CommandID:   commandID,
		Payload:     payload,
		Status:      "pending",
		Attempts:    0,
		AvailableAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(task); err != nil {
		return nil, err
	}
	return task, nil
}
