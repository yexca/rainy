package subsonic

import (
	"context"
	"strings"

	"rainy/internal/auth"
	"rainy/internal/model"
)

// subsonicUser maps a Rainy user to Subsonic roles: adminRole = isAdmin,
// settingsRole/playlistRole/streamRole = true, downloadRole = canDownload,
// uploadRole/coverArtRole = canEdit (admin or manager); other roles false.
func subsonicUser(u *model.User, folders []int64) User {
	return User{
		Username:          u.Username,
		Email:             u.Email,
		ScrobblingEnabled: true,
		AdminRole:         u.IsAdmin,
		SettingsRole:      true,
		DownloadRole:      u.CanDownload,
		UploadRole:        u.CanEdit(),
		PlaylistRole:      true,
		CoverArtRole:      u.CanEdit(),
		StreamRole:        true,
		Folders:           folders,
	}
}

// folderIDs lists every library id (all users can access every library).
func (a *API) folderIDs(ctx context.Context) ([]int64, error) {
	libs, err := a.app.Store.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(libs))
	for _, l := range libs {
		ids = append(ids, l.ID)
	}
	return ids, nil
}

// getUser returns a user; non-admins may only look themselves up.
func (a *API) getUser(q *request) (*Response, error) {
	name, err := q.requiredStr("username")
	if err != nil {
		return nil, err
	}
	u := q.user
	if !strings.EqualFold(name, q.user.Username) {
		if !q.user.IsAdmin {
			return nil, errForbidden()
		}
		if u, err = a.app.Store.GetUserByUsername(q.ctx, name); err != nil {
			return nil, notFoundAs(err, "User")
		}
	}
	folders, err := a.folderIDs(q.ctx)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	su := subsonicUser(u, folders)
	resp.User = &su
	return resp, nil
}

func (a *API) getUsers(q *request) (*Response, error) {
	if !q.user.IsAdmin {
		return nil, errForbidden()
	}
	users, err := a.app.Store.ListUsers(q.ctx)
	if err != nil {
		return nil, err
	}
	folders, err := a.folderIDs(q.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(users))
	for i := range users {
		out = append(out, subsonicUser(&users[i], folders))
	}
	resp := newResponse()
	resp.Users = &Users{Users: out}
	return resp, nil
}

// passwordParam reads a required password (plain or "enc:<hex>").
func passwordParam(q *request) (string, error) {
	raw := q.params.Get("password")
	if raw == "" {
		return "", errMissing("password")
	}
	pw, ok := decodePassword(raw)
	if !ok {
		return "", newError(codeGeneric, "Invalid password encoding")
	}
	return pw, nil
}

// applyRoles sets the Rainy permission flags from Subsonic role parameters that are
// present (uploadRole or coverArtRole → canManage).
func applyRoles(q *request, u *model.User) error {
	if v, err := q.optBool("adminRole"); err != nil {
		return err
	} else if v != nil {
		u.IsAdmin = *v
	}
	if v, err := q.optBool("downloadRole"); err != nil {
		return err
	} else if v != nil {
		u.CanDownload = *v
	}
	upload, err := q.optBool("uploadRole")
	if err != nil {
		return err
	}
	cover, err := q.optBool("coverArtRole")
	if err != nil {
		return err
	}
	if upload != nil || cover != nil {
		u.CanManage = (upload != nil && *upload) || (cover != nil && *cover)
	}
	return nil
}

func (a *API) createUser(q *request) (*Response, error) {
	if !q.user.IsAdmin {
		return nil, errForbidden()
	}
	name, err := q.requiredStr("username")
	if err != nil {
		return nil, err
	}
	pw, err := passwordParam(q)
	if err != nil {
		return nil, err
	}
	u := &model.User{Username: name, Email: q.str("email"), CanDownload: true}
	if err := applyRoles(q, u); err != nil {
		return nil, err
	}
	if err := a.app.Auth.CreateUser(q.ctx, u, pw); err != nil {
		return nil, err
	}
	return newResponse(), nil
}

// updateUser changes email, password and roles of a user. Admins cannot remove their own
// admin role.
func (a *API) updateUser(q *request) (*Response, error) {
	if !q.user.IsAdmin {
		return nil, errForbidden()
	}
	name, err := q.requiredStr("username")
	if err != nil {
		return nil, err
	}
	u, err := a.app.Store.GetUserByUsername(q.ctx, name)
	if err != nil {
		return nil, notFoundAs(err, "User")
	}
	if q.has("email") {
		u.Email = q.str("email")
	}
	if err := applyRoles(q, u); err != nil {
		return nil, err
	}
	if u.ID == q.user.ID && !u.IsAdmin {
		return nil, newError(codeGeneric, "You cannot remove your own admin role")
	}
	// Validate everything before changing anything, so a rejected password does not leave
	// the roles half-applied (and vice versa).
	var newPassword string
	if q.params.Get("password") != "" {
		if newPassword, err = passwordParam(q); err != nil {
			return nil, err
		}
		if err := auth.ValidatePassword(newPassword); err != nil {
			return nil, err
		}
	}
	if err := a.app.Store.UpdateUser(q.ctx, u); err != nil {
		return nil, err
	}
	if newPassword != "" {
		if err := a.app.Auth.ChangePassword(q.ctx, u.ID, newPassword); err != nil {
			return nil, err
		}
	}
	return newResponse(), nil
}

func (a *API) deleteUser(q *request) (*Response, error) {
	if !q.user.IsAdmin {
		return nil, errForbidden()
	}
	name, err := q.requiredStr("username")
	if err != nil {
		return nil, err
	}
	u, err := a.app.Store.GetUserByUsername(q.ctx, name)
	if err != nil {
		return nil, notFoundAs(err, "User")
	}
	if u.ID == q.user.ID {
		return nil, newError(codeGeneric, "You cannot delete yourself")
	}
	if err := a.app.Store.DeleteUser(q.ctx, u.ID); err != nil {
		return nil, err
	}
	return newResponse(), nil
}

// changePassword changes the caller's password, or any user's for admins.
func (a *API) changePassword(q *request) (*Response, error) {
	name, err := q.requiredStr("username")
	if err != nil {
		return nil, err
	}
	pw, err := passwordParam(q)
	if err != nil {
		return nil, err
	}
	target := q.user
	if !strings.EqualFold(name, q.user.Username) {
		if !q.user.IsAdmin {
			return nil, errForbidden()
		}
		if target, err = a.app.Store.GetUserByUsername(q.ctx, name); err != nil {
			return nil, notFoundAs(err, "User")
		}
	}
	if err := a.app.Auth.ChangePassword(q.ctx, target.ID, pw); err != nil {
		return nil, err
	}
	return newResponse(), nil
}
