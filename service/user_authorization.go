package service

import "github.com/QuantumNous/new-api/common"

type UserIdentity struct {
	ID   int
	Role int
}

func CanManageUser(actor UserIdentity, target UserIdentity) bool {
	if actor.ID == 1 && actor.Role == common.RoleRootUser {
		return true
	}
	if actor.ID == target.ID || actor.Role < common.RoleAdminUser {
		return false
	}
	return actor.Role > target.Role
}
