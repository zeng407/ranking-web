package admin

import (
	"context"
	"strings"
)

// User is one account as the moderation list draws it.
type User struct {
	ID        int64
	Name      string
	Email     string
	AvatarURL string
	Roles     []string
	PostCount int
	CreatedAt string
}

// UserPage is one page of the account list.
type UserPage struct {
	Users   []User
	Total   int
	Page    int
	PerPage int
}

// Users lists accounts, newest first, optionally filtered by name or address.
//
// The original had two endpoints — an index and a search that answered an empty page for
// an empty keyword. One here: an empty keyword is the index, which is what a client with
// a search box that starts empty actually needs.
func (service *Service) Users(ctx context.Context, keyword string, page int) (UserPage, error) {
	if page < 1 {
		page = 1
	}
	users, total, err := service.store.ListUsers(ctx, strings.TrimSpace(keyword), page, UsersPerPage)
	if err != nil {
		return UserPage{}, wrap("list users", err)
	}
	return UserPage{Users: users, Total: total, Page: page, PerPage: UsersPerPage}, nil
}

// BanUser adds the banned role, drops the account's sessions and invalidates the role
// cache Laravel reads.
func (service *Service) BanUser(ctx context.Context, userID int64) error {
	roles, err := service.store.UserRoles(ctx, userID)
	if err != nil {
		return wrap("read the account's roles", err)
	}
	for _, role := range roles {
		if role == AdminRoleSlug {
			return ErrCannotBanAdministrator
		}
	}

	if err := service.store.AddRole(ctx, userID, BannedRoleSlug); err != nil {
		return wrap("add the banned role", err)
	}
	service.forgetRoleCache(ctx, userID)
	// After the role is written, never before: a session revoked first could be replaced
	// by a fresh login in the window before the ban lands.
	service.revokeSessions(ctx, userID)
	return nil
}

// UnbanUser removes the banned role and invalidates the role cache.
//
// Sessions are not restored, and cannot be: the ban revoked them. The account signs in
// again.
func (service *Service) UnbanUser(ctx context.Context, userID int64) error {
	if _, err := service.store.UserRoles(ctx, userID); err != nil {
		return wrap("read the account's roles", err)
	}
	if err := service.store.RemoveRole(ctx, userID, BannedRoleSlug); err != nil {
		return wrap("remove the banned role", err)
	}
	service.forgetRoleCache(ctx, userID)
	return nil
}

// forgetRoleCache clears Laravel's cached role list.
//
// A failure is logged by the caller's error path nowhere and swallowed here on purpose:
// the pivot is already written, which is the record that matters, and the cache expires
// within the hour. Refusing the ban because a cache delete failed would leave the account
// unbanned everywhere.
func (service *Service) forgetRoleCache(ctx context.Context, userID int64) {
	if service.roleCache == nil {
		return
	}
	_ = service.roleCache.ForgetUserRoles(ctx, userID)
}

func (service *Service) revokeSessions(ctx context.Context, userID int64) {
	if service.sessions == nil {
		return
	}
	_, _ = service.sessions.RevokeAll(ctx, userID)
}
