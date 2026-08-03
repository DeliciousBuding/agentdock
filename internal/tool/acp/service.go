package acp

import acpruntime "github.com/uvwt/agentdock/internal/acp"

type Service struct {
	manager *acpruntime.Manager
}

func New(manager *acpruntime.Manager) *Service {
	return &Service{manager: manager}
}

func (s *Service) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.Close()
}
