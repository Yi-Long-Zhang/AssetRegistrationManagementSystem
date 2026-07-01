package tests

import (
	"testing"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

func TestTransitionAllowsValidRoleAndStatus(t *testing.T) {
	next, err := service.Transition("approve", model.TicketStatusPendingApproval, model.RoleApprover)
	if err != nil {
		t.Fatalf("expected transition to succeed: %v", err)
	}
	if next != model.TicketStatusPendingApproval {
		t.Fatalf("expected %s, got %s", model.TicketStatusPendingApproval, next)
	}
}

func TestTransitionRejectsInvalidStatus(t *testing.T) {
	if _, err := service.Transition("approve", model.TicketStatusDraft, model.RoleApprover); err == nil {
		t.Fatal("expected invalid status to fail")
	}
}

func TestTransitionRejectsInvalidRole(t *testing.T) {
	if _, err := service.Transition("start", model.TicketStatusApproved, model.RoleApplicant); err == nil {
		t.Fatal("expected invalid role to fail")
	}
}
