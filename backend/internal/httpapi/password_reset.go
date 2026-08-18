package httpapi

import "net/http"

// Forgot password and reset, from Auth\ForgotPasswordController and
// Auth\ResetPasswordController. These two were the last endpoints the SPA still left to
// Laravel, because they need to send mail.

type forgotPasswordRequest struct {
	Email string `json:"email"`
	// Locale picks the language of the mail. The SPA sends the one the visitor is
	// reading; an unknown or absent value gets the site's own language.
	Locale string `json:"locale"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// forgotPassword answers 200 whether or not the address has an account.
//
// The response is deliberately uninformative, and that is not a shortcut — see
// auth.RequestPasswordReset. The page it drives says "if this address has an account, the
// mail is on its way", which is true in every case it can answer.
func (a *api) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuthService(w, r) {
		return
	}

	var request forgotPasswordRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := a.authService.RequestPasswordReset(
		r.Context(), request.Email, request.Locale, a.clientInfo(r)); err != nil {
		a.writeAccountError(w, r, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, r, http.StatusOK, envelope{Data: map[string]any{"status": "sent"}})
}

// resetPassword sets the new password and signs the caller in.
//
// The success path is the login response, cookies included, because finishing a reset is
// a login: the caller proved control of the address on file. See auth.ResetPassword.
func (a *api) resetPassword(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuthService(w, r) {
		return
	}

	var request resetPasswordRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	grant, err := a.authService.ResetPassword(
		r.Context(), request.Token, request.NewPassword, a.clientInfo(r))
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}

	a.writeGrant(w, r, grant)
}
