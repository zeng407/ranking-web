package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The admin bundle's own files, served from behind the admin role.
//
// THE BACK OFFICE'S JAVASCRIPT IS NOT PUBLIC. Laravel served the admin screens as Blade
// views behind a middleware, so their markup was never a file anyone could fetch. Building
// them into the public SPA bundle instead would publish every admin screen, endpoint path
// and field name to every visitor — the API would still refuse a non-moderator's request,
// but the map of the back office would be readable by anyone with the URL of a .js file on
// the CDN. So the bundle is built to its own directory, kept off the public origin, and
// served by this process only to a request that carries proof of the admin role.
//
// A browser cannot put an Authorization header on a <script src> or on a top-level
// navigation, so the proof has to be a cookie. It is minted by adminAssetGrant, which is a
// normal Bearer-authenticated admin endpoint, and it authorizes nothing but reading these
// files: every piece of data the screens show still needs the access token.

const (
	// adminAssetPrefix is where the bundle is mounted, and the cookie's Path: a cookie
	// scoped here is not sent with any other request.
	adminAssetPrefix = "/admin/"
	// adminAssetCookieName is the gate pass.
	adminAssetCookieName = "2pick_admin"
	// adminAssetTTL is how long one pass lasts. Short, because it is renewed on the way
	// through — see serveAdminAsset — so a moderator who keeps the tab open keeps working,
	// and one who closes it stops being able to fetch the bundle within the hour.
	adminAssetTTL = time.Hour
)

// adminAssetCookieLabel separates the pass's signing key from every other use of the
// token signing seed, so a signature produced here can never be replayed as one of those.
const adminAssetCookieLabel = "2pick admin asset cookie v1"

// AdminAssetKey derives the pass's signing key from the token signing key's seed.
//
// Deliberately not a variable of its own: a separate ADMIN_ASSET_SECRET would be one more
// secret to distribute, and a deployment that forgot it would hand out passes signed with
// an empty key. Tying it to the seed means the back office is loadable exactly where
// sign-in works, and rotating the signing key invalidates outstanding passes too.
func AdminAssetKey(seed []byte) []byte {
	if len(seed) == 0 {
		return nil
	}
	sum := sha256.Sum256(append([]byte(adminAssetCookieLabel), seed...))
	return sum[:]
}

// adminAssetGrant hands the current admin session a pass for the bundle.
//
// POST rather than GET: it writes a cookie, and a GET that changes state is a GET a
// prefetcher can fire. The SPA's public shell calls this after signing in and before
// navigating to the back office.
func (a *api) adminAssetGrant(w http.ResponseWriter, r *http.Request) {
	if !a.adminAssetsConfigured(w, r) {
		return
	}
	userID, ok := a.callerUserID(w, r)
	if !ok {
		return
	}

	a.setAdminAssetCookie(w, r, userID)
	// No body: the cookie is the whole result, and naming the bundle's path here would
	// invite the client to hard-code a second copy of it.
	a.writeNoContent(w)
}

// adminAssetRevoke drops the pass, for a moderator signing out.
//
// Clearing it is not what stops a revoked admin from reading the bundle — the pass expires
// on its own within the hour and cannot be renewed once the account loses the role — but
// leaving it behind on a shared machine is worth avoiding.
func (a *api) adminAssetRevoke(w http.ResponseWriter, r *http.Request) {
	if !a.adminAssetsConfigured(w, r) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminAssetCookieName,
		Value:    "",
		Path:     adminAssetPrefix,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   a.cookiesAreSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
	a.writeNoContent(w)
}

// serveAdminAsset serves one file out of the bundle, to a request that carries a valid
// pass and to nothing else.
//
// A request without one gets 403 and no hint about what is in the directory. The SPA's
// public shell is what recovers from that: it holds the sign-in page, calls
// adminAssetGrant, and navigates here again.
func (a *api) serveAdminAsset(w http.ResponseWriter, r *http.Request) {
	if a.adminAssetDir == "" {
		// Nothing is mounted, so this path is not a resource on this server at all.
		a.notFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	userID, expiresAt, ok := a.verifyAdminAssetCookie(r)
	if !ok {
		// 403, not 404: the moderator whose pass has expired needs to be able to tell
		// "sign in again" apart from "this build has no such file".
		writeError(w, r, http.StatusForbidden, "admin_assets_forbidden",
			"a moderator session is required to load the back office")
		return
	}
	// Renewed on the way through, so an open tab keeps working past the hour without the
	// bundle it is running being what proves the role: the renewal only extends a pass
	// that is still valid, and a pass cannot be created here.
	if expiresAt.Sub(a.now()) < adminAssetTTL/2 {
		a.setAdminAssetCookie(w, r, userID)
	}

	requested := strings.TrimPrefix(r.URL.Path, adminAssetPrefix)
	file, info, err := a.openAdminAsset(requested)
	if err != nil {
		a.notFound(w, r)
		return
	}
	defer file.Close()

	// private, no-store on the whole bundle. A shared cache in front of this process must
	// never keep a copy it could hand to a request without a pass, and the files are small
	// enough that the round trip costs less than that risk.
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Cloudflare-CDN-Cache-Control", "no-store")
	// The bundle is never framed, and never anybody else's origin's script.
	w.Header().Set("X-Frame-Options", "DENY")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// openAdminAsset resolves a request path inside the bundle directory.
//
// Directories are never listed and never served: a request for one, or for a path that does
// not exist, falls back to index.html, because the back office is a single-page app whose
// deep links are its own routes rather than files. The fallback is the reason this is not
// http.FileServer, which would answer a directory with a listing.
func (a *api) openAdminAsset(requested string) (*os.File, fs.FileInfo, error) {
	// path.Clean on a rooted path collapses every ".." before the join, so a request for
	// /admin/../../etc/passwd cannot leave the directory.
	clean := path.Clean("/" + requested)
	candidate := filepath.Join(a.adminAssetDir, filepath.FromSlash(clean))

	file, info, err := openRegularFile(candidate)
	if err == nil {
		return file, info, nil
	}
	if clean == "/index.html" {
		// The fallback itself is missing, which is a broken deployment rather than a
		// missing route.
		return nil, nil, err
	}
	return openRegularFile(filepath.Join(a.adminAssetDir, "index.html"))
}

func openRegularFile(name string) (*os.File, fs.FileInfo, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		if err == nil {
			err = errors.New("httpapi: not a regular file")
		}
		return nil, nil, err
	}
	return file, info, nil
}

// The pass is `<user id>.<expiry unix>.<hmac>`, signed with a key this process derives from
// the token signing key. It is not a session and cannot be turned into one: nothing reads
// the user id out of it but the renewal above, and no data endpoint accepts it.
func (a *api) setAdminAssetCookie(w http.ResponseWriter, r *http.Request, userID int64) {
	expiresAt := a.now().Add(adminAssetTTL)
	payload := strconv.FormatInt(userID, 10) + "." + strconv.FormatInt(expiresAt.Unix(), 10)

	http.SetCookie(w, &http.Cookie{
		Name:    adminAssetCookieName,
		Value:   payload + "." + a.signAdminAsset(payload),
		Path:    adminAssetPrefix,
		Expires: expiresAt,
		MaxAge:  int(adminAssetTTL.Seconds()),
		// httpOnly: no script has any reason to read it, and the bundle's own scripts are
		// what an XSS on the admin origin would be running.
		HttpOnly: true,
		Secure:   a.cookiesAreSecure(r),
		// Lax rather than Strict, because loading the bundle starts with a top-level
		// navigation from the public shell, and Strict would drop the cookie on it. Lax
		// still withholds it from cross-site subresource requests, and the pass authorizes
		// reading static files rather than any state change.
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *api) verifyAdminAssetCookie(r *http.Request) (int64, time.Time, bool) {
	if len(a.adminAssetKey) == 0 {
		return 0, time.Time{}, false
	}
	cookie, err := r.Cookie(adminAssetCookieName)
	if err != nil || cookie.Value == "" {
		return 0, time.Time{}, false
	}

	lastDot := strings.LastIndexByte(cookie.Value, '.')
	if lastDot < 0 {
		return 0, time.Time{}, false
	}
	payload, signature := cookie.Value[:lastDot], cookie.Value[lastDot+1:]
	if !hmac.Equal([]byte(signature), []byte(a.signAdminAsset(payload))) {
		return 0, time.Time{}, false
	}

	parts := strings.Split(payload, ".")
	if len(parts) != 2 {
		return 0, time.Time{}, false
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || userID <= 0 {
		return 0, time.Time{}, false
	}
	seconds, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, time.Time{}, false
	}
	expiresAt := time.Unix(seconds, 0)
	if !a.now().Before(expiresAt) {
		return 0, time.Time{}, false
	}
	return userID, expiresAt, true
}

func (a *api) signAdminAsset(payload string) string {
	mac := hmac.New(sha256.New, a.adminAssetKey)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// adminAssetsConfigured refuses the grant endpoints when no bundle is mounted, so a
// deployment without one does not hand out passes to a directory that is not there.
func (a *api) adminAssetsConfigured(w http.ResponseWriter, r *http.Request) bool {
	if a.adminAssetDir == "" || len(a.adminAssetKey) == 0 {
		a.notFound(w, r)
		return false
	}
	return true
}
