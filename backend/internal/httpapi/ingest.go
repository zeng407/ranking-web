package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	"2pick.app/backend/internal/ingest"
)

// Adding media to a post, from Api\ElementController::createMedia.

// IngestService is the slice of ingest.Service this layer uses.
type IngestService interface {
	Upload(ctx context.Context, userID int64, serial, fileName string, content []byte) (ingest.Stored, error)
	AddURLs(ctx context.Context, userID int64, serial, list string) (ingest.BatchResult, error)
	ReplaceMedia(ctx context.Context, userID, elementID int64, fileName string, content []byte) (ingest.Stored, error)
}

// maxUploadRequestBytes bounds the whole multipart body: the file plus its part headers.
// The file itself is checked against ingest.MaxFileBytes.
const maxUploadRequestBytes = ingest.MaxFileBytes + (64 << 10)

type uploadedElementResponse struct {
	ID        int64  `json:"id"`
	SourceURL string `json:"source_url"`
	ThumbURL  string `json:"thumb_url"`
	Title     string `json:"title"`
	Type      string `json:"type"`
}

// uploadPostElement takes a multipart form with one file part named file.
//
// The part name matches the original's, so the old editor's uploader would work against
// this endpoint unchanged if it were ever pointed at it.
func (a *api) uploadPostElement(w http.ResponseWriter, r *http.Request) {
	if a.ingest == nil {
		writeError(w, r, http.StatusServiceUnavailable, "uploads_not_configured",
			"media uploads are not configured on this server")
		return
	}
	userID, ok := a.callerUserID(w, r)
	if !ok {
		return
	}

	fileName, content, ok := readUploadedFile(w, r)
	if !ok {
		return
	}

	stored, err := a.ingest.Upload(r.Context(), userID, r.PathValue("serial"), fileName, content)
	if err != nil {
		a.writeIngestError(w, r, err)
		return
	}

	writePrivateJSON(w, r, http.StatusCreated, uploadedElementResponse{
		ID: stored.ID, SourceURL: stored.SourceURL, ThumbURL: stored.ThumbURL,
		Title: stored.Title, Type: stored.Type,
	})
}

// readUploadedFile pulls the one file part named file out of a multipart body.
func readUploadedFile(w http.ResponseWriter, r *http.Request) (string, []byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_upload",
			"a multipart form with one file part named file is required")
		return "", nil, false
	}
	defer file.Close()

	if header.Size > ingest.MaxFileBytes {
		// Refused before reading, on the size the part declares.
		writeFieldErrors(w, r, map[string][]string{"file": {ingest.CodeTooLarge}})
		return "", nil, false
	}
	// One byte past the limit, so a part that lied about its size is still caught.
	content, err := io.ReadAll(io.LimitReader(file, ingest.MaxFileBytes+1))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_upload", "the upload could not be read")
		return "", nil, false
	}
	return header.Filename, content, true
}

// replaceElementMedia swaps the file behind an element without disturbing anything else
// about it — its title, its place in the post, and its votes all stay.
//
// Laravel needed two requests for this (POST .../upload for a `path_id`, then PUT with it);
// see ingest.Service.ReplaceMedia for why one is enough here. 200, not 201: no element is
// created.
func (a *api) replaceElementMedia(w http.ResponseWriter, r *http.Request) {
	if a.ingest == nil {
		writeError(w, r, http.StatusServiceUnavailable, "uploads_not_configured",
			"media uploads are not configured on this server")
		return
	}
	userID, ok := a.callerUserID(w, r)
	if !ok {
		return
	}
	elementID, ok := editorElementID(w, r)
	if !ok {
		return
	}

	fileName, content, ok := readUploadedFile(w, r)
	if !ok {
		return
	}

	stored, err := a.ingest.ReplaceMedia(r.Context(), userID, elementID, fileName, content)
	if err != nil {
		a.writeIngestError(w, r, err)
		return
	}

	writePrivateJSON(w, r, http.StatusOK, uploadedElementResponse{
		ID: stored.ID, SourceURL: stored.SourceURL, ThumbURL: stored.ThumbURL,
		Title: stored.Title, Type: stored.Type,
	})
}

type addURLsRequest struct {
	// URLs is the original's field: comma-separated, each entry optionally "url title".
	// A YouTube embed is the whole value rather than one entry, because it is a block of
	// HTML full of spaces.
	URLs string `json:"urls"`
}

type addedElementsResponse struct {
	Added []uploadedElementResponse `json:"added"`
	// Failed names each URL that did not become an element and why: unrecognised,
	// unavailable, too_large or post_full.
	Failed []failedURLResponse `json:"failed"`
}

type failedURLResponse struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// maxURLListBytes bounds the pasted list. A hundred URLs at a generous 2 KiB each is far
// more than any real paste, and the count itself is checked by the service.
const maxURLListBytes = 256 << 10

// addPostElementsByURL turns a pasted list of URLs into elements.
//
// 207, not 201: a batch normally succeeds in part. One dead link out of thirty is the
// ordinary case, and answering 201 would say everything worked while 422 would say
// nothing did. The body names each failure.
func (a *api) addPostElementsByURL(w http.ResponseWriter, r *http.Request) {
	if a.ingest == nil {
		writeError(w, r, http.StatusServiceUnavailable, "uploads_not_configured",
			"media uploads are not configured on this server")
		return
	}
	userID, ok := a.callerUserID(w, r)
	if !ok {
		return
	}

	var request addURLsRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxURLListBytes)
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	result, err := a.ingest.AddURLs(r.Context(), userID, r.PathValue("serial"), request.URLs)
	if err != nil {
		a.writeIngestError(w, r, err)
		return
	}

	response := addedElementsResponse{
		Added:  make([]uploadedElementResponse, 0, len(result.Added)),
		Failed: make([]failedURLResponse, 0, len(result.Failed)),
	}
	for _, stored := range result.Added {
		response.Added = append(response.Added, uploadedElementResponse{
			ID: stored.ID, SourceURL: stored.SourceURL, ThumbURL: stored.ThumbURL,
			Title: stored.Title, Type: stored.Type,
		})
	}
	for _, failure := range result.Failed {
		response.Failed = append(response.Failed, failedURLResponse{
			URL: failure.URL, Reason: failure.Reason,
		})
	}

	status := http.StatusMultiStatus
	if len(response.Failed) == 0 {
		status = http.StatusCreated
	}
	writePrivateJSON(w, r, status, response)
}

// writeIngestError renders an upload's failures.
//
// A post that is not the caller's answers 404, the same as everywhere else in the editor:
// a 403 would confirm the serial exists.
func (a *api) writeIngestError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *ingest.ErrInvalid
	if errors.As(err, &invalid) {
		writeFieldErrors(w, r, invalid.Fields)
		return
	}
	if errors.Is(err, ingest.ErrPostNotFound) || errors.Is(err, ingest.ErrElementNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	if errors.Is(err, ingest.ErrNotConfigured) {
		a.logger.Info("upload_not_configured", "reason", err)
		writeError(w, r, http.StatusServiceUnavailable, "uploads_not_configured",
			"media uploads are not configured on this server")
		return
	}
	a.logger.Error("upload_failed", "error", err)
	writeError(w, r, http.StatusInternalServerError, "internal_error",
		"the request could not be completed")
}
