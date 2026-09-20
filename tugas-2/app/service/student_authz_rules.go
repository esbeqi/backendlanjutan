package service

import (
	"api-students/app/model"
	"api-students/helper"
)

func CanAccessStudent(
	current model.AuthUser,
	ownerID int,
	perms *helper.PermissionSet,
	anyPermission string,
) bool {
	if current.UserID == ownerID {
		return true
	}

	return perms.Can(current.Role, anyPermission)
}
