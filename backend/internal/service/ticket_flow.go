package service

import (
	"fmt"

	"asset-registration-management-system/backend/internal/model"
)

var transitions = map[string]struct {
	From  model.TicketStatus
	To    model.TicketStatus
	Roles []model.Role
}{
	"submit":   {From: model.TicketStatusDraft, To: model.TicketStatusSubmitted, Roles: []model.Role{model.RoleAdmin, model.RoleApplicant}},
	"approve":  {From: model.TicketStatusSubmitted, To: model.TicketStatusApproved, Roles: []model.Role{model.RoleAdmin, model.RoleApprover}},
	"reject":   {From: model.TicketStatusSubmitted, To: model.TicketStatusRejected, Roles: []model.Role{model.RoleAdmin, model.RoleApprover}},
	"start":    {From: model.TicketStatusApproved, To: model.TicketStatusInProgress, Roles: []model.Role{model.RoleAdmin, model.RoleAssetManager}},
	"complete": {From: model.TicketStatusInProgress, To: model.TicketStatusDone, Roles: []model.Role{model.RoleAdmin, model.RoleAssetManager}},
	"close":    {From: model.TicketStatusDone, To: model.TicketStatusClosed, Roles: []model.Role{model.RoleAdmin, model.RoleApplicant, model.RoleAssetManager}},
	"cancel":   {From: model.TicketStatusDraft, To: model.TicketStatusCancelled, Roles: []model.Role{model.RoleAdmin, model.RoleApplicant}},
}

func Transition(action string, current model.TicketStatus, role model.Role) (model.TicketStatus, error) {
	rule, ok := transitions[action]
	if !ok {
		return "", fmt.Errorf("unsupported action %q", action)
	}
	if current != rule.From {
		return "", fmt.Errorf("ticket status %q cannot %s", current, action)
	}
	for _, allowed := range rule.Roles {
		if role == allowed {
			return rule.To, nil
		}
	}
	return "", fmt.Errorf("role %q cannot %s ticket", role, action)
}
