package httpapi

import (
	"errors"
	"io"
	"net/http"
	"time"

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/media"
)

// The account settings endpoints, from Profile\ProfileController.
//
// All four require a bearer token and act on the account the token names — none of them
// takes a user id, so there is no object to authorize against.

// maxAvatarRequestBytes bounds the whole multipart body, which is the avatar plus the
// part headers. The image itself is checked against auth.MaxAvatarBytes; this is the
// outer limit that stops a request being read at all.
const maxAvatarRequestBytes = auth.MaxAvatarBytes + (64 << 10)

type accountResponse struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	// HasPassword tells the settings form which of the two password endpoints to use.
	HasPassword  bool `json:"has_password"`
	GoogleLinked bool `json:"google_linked"`
	// NameChangeAllowedAt is when the name may next change, RFC 3339, or absent when it
	// may change now. Serving the moment rather than a boolean lets the form say when
	// instead of only that it cannot.
	NameChangeAllowedAt string `json:"name_change_allowed_at,omitempty"`
}

type changeNameRequest struct {
	Name string `json:"name"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type initialPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// accountProfile is one resource with two verbs: read the settings, or rename.
func (a *api) accountProfile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.accountSettings(w, r)
	case http.MethodPut:
		a.changeAccountName(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, PUT")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// accountPassword splits by verb rather than by path.
//
// PUT replaces a password after proving the current one. POST creates the first one, for
// an account whose password column is empty — Laravel had that at account/password/init,
// but the difference is create-versus-replace, which is what the verb is for.
func (a *api) accountPassword(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		a.changeAccountPassword(w, r)
	case http.MethodPost:
		a.setInitialAccountPassword(w, r)
	default:
		w.Header().Set("Allow", "POST, PUT")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// accountCaller resolves the account, and refuses when the session service — which the
// settings need in order to re-issue a session after a password change — is absent.
func (a *api) accountCaller(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if a.authService == nil {
		writeError(w, r, http.StatusServiceUnavailable, "auth_not_configured",
			"account settings are not configured")
		return 0, false
	}
	return a.callerUserID(w, r)
}

func (a *api) accountSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.accountCaller(w, r)
	if !ok {
		return
	}

	account, err := a.authService.Account(r.Context(), userID)
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	a.writeAccount(w, r, account)
}

func (a *api) changeAccountName(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.accountCaller(w, r)
	if !ok {
		return
	}

	var request changeNameRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	account, err := a.authService.ChangeName(r.Context(), userID, request.Name)
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	a.writeAccount(w, r, account)
}

// uploadAccountAvatar takes a multipart form with one file part named avatar.
//
// Multipart rather than a JSON body with base64: the file is up to 4 MiB and base64
// would carry a third more bytes for no gain, and the existing element upload path is
// multipart too.
func (a *api) uploadAccountAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.accountCaller(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarRequestBytes)
	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_upload",
			"a multipart form with one file part named avatar is required")
		return
	}
	defer file.Close()

	if header.Size > auth.MaxAvatarBytes {
		// Refused before reading, when the part declares its size.
		writeFieldErrors(w, r, auth.FieldErrors{"avatar": []string{auth.CodeTooLarge}})
		return
	}
	// One byte past the limit, so a part that lied about its size is still caught.
	image, err := io.ReadAll(io.LimitReader(file, auth.MaxAvatarBytes+1))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_upload", "the upload could not be read")
		return
	}

	url, err := a.authService.UploadAvatar(r.Context(), userID, image, media.AvatarKey)
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}

	writePrivateJSON(w, r, http.StatusOK, map[string]string{"avatar_url": url})
}

func (a *api) changeAccountPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.accountCaller(w, r)
	if !ok {
		return
	}

	var request changePasswordRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	grant, err := a.authService.ChangePassword(
		r.Context(), userID, request.CurrentPassword, request.NewPassword, a.clientInfo(r))
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	// A fresh session, because the change ended every session this account held —
	// including the caller's. Answering with the grant keeps them signed in here.
	a.writeGrant(w, r, grant)
}

func (a *api) setInitialAccountPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.accountCaller(w, r)
	if !ok {
		return
	}

	var request initialPasswordRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	grant, err := a.authService.SetInitialPassword(
		r.Context(), userID, request.NewPassword, a.clientInfo(r))
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	a.writeGrant(w, r, grant)
}

// callerUserID resolves the account the bearer token names.
//
// requireAuth has already verified the token, so a subject that is not a user id here
// means the token was issued for something that is not an account — refused rather than
// coerced, because every handler that uses this writes to a row chosen by this number.
//
// It asks nothing about which service is configured: the post editor uses it too, and
// that one has no need of the session service at all.
func (a *api) callerUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return 0, false
	}
	userID, err := auth.SubjectToUserID(identity.Subject)
	if err != nil {
		a.logger.Warn("identity_subject_is_not_a_user_id", "subject", identity.Subject)
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return 0, false
	}
	return userID, true
}

func (a *api) writeAccount(w http.ResponseWriter, r *http.Request, account auth.Account) {
	response := accountResponse{
		Name:         account.Name,
		Email:        account.Email,
		AvatarURL:    account.AvatarURL,
		HasPassword:  account.HasPassword,
		GoogleLinked: account.GoogleLinked,
	}
	if allowedAt := a.authService.NameChangeAllowedAt(account); allowedAt.After(time.Now()) {
		response.NameChangeAllowedAt = allowedAt.Format(time.RFC3339)
	}
	writePrivateJSON(w, r, http.StatusOK, response)
}

// writeAccountError renders the settings failures.
//
// Field errors get the same 422 shape as registration, so one client-side renderer
// covers every form. ErrUserNotFound is a 401 rather than a 404: the token verified, so
// the account it names was deleted underneath it, and the caller's session is what is
// actually wrong.
func (a *api) writeAccountError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *auth.ErrAccountInvalid
	if errors.As(err, &invalid) {
		writeFieldErrors(w, r, invalid.Fields)
		return
	}
	if errors.Is(err, auth.ErrUserNotFound) {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}
	// Not a fault: an api started without the object-store variables cannot accept an
	// avatar, and answering 500 would have a client retrying something that will never
	// work. Logged at info because the operator chose this by not configuring it.
	if errors.Is(err, auth.ErrNotConfigured) {
		a.logger.Info("account_operation_not_configured", "reason", err)
		writeError(w, r, http.StatusServiceUnavailable, "account_not_configured",
			"this operation is not configured on this server")
		return
	}
	a.logger.Error("account_request_failed", "error", err)
	writeError(w, r, http.StatusInternalServerError, "internal_error",
		"the request could not be completed")
}
