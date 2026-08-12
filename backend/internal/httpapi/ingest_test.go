package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/ingest"
)

// The upload endpoint's transport.

type fakeIngest struct {
	stored   ingest.Stored
	err      error
	calls    int
	lastName string
	lastBody []byte
	lastPost string
	lastList string
	urlCalls int
	urlErr   error
	batch    ingest.BatchResult

	replaceCalls  int
	lastElementID int64
}

func (service *fakeIngest) AddURLs(
	_ context.Context, _ int64, serial, list string,
) (ingest.BatchResult, error) {
	service.urlCalls++
	service.lastPost, service.lastList = serial, list
	if service.urlErr != nil {
		return ingest.BatchResult{}, service.urlErr
	}
	return service.batch, nil
}

func (service *fakeIngest) Upload(
	_ context.Context, _ int64, serial, fileName string, content []byte,
) (ingest.Stored, error) {
	service.calls++
	service.lastPost, service.lastName, service.lastBody = serial, fileName, content
	if service.err != nil {
		return ingest.Stored{}, service.err
	}
	return service.stored, nil
}

func (service *fakeIngest) ReplaceMedia(
	_ context.Context, _, elementID int64, fileName string, content []byte,
) (ingest.Stored, error) {
	service.replaceCalls++
	service.lastElementID, service.lastName, service.lastBody = elementID, fileName, content
	if service.err != nil {
		return ingest.Stored{}, service.err
	}
	return service.stored, nil
}

func newFakeIngest() *fakeIngest {
	return &fakeIngest{stored: ingest.Stored{
		ID: 5, SourceURL: "https://file.2pick.test/abcdefgh/a.png",
		ThumbURL: "https://file.2pick.test/abcdefgh/a.png", Title: "a", Type: "image",
	}}
}

func ingestHandler(service IngestService) http.Handler {
	return New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ingest:       service,
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42", Roles: []string{}}},
	})
}

func upload(t *testing.T, fieldName, fileName string, content []byte) *http.Request {
	t.Helper()
	return multipartRequest(t, http.MethodPost,
		"/api/v1/account/posts/abcdefgh/elements/uploads", fieldName, fileName, content)
}

func replacement(t *testing.T, target, fileName string, content []byte) *http.Request {
	t.Helper()
	return multipartRequest(t, http.MethodPut, target, "file", fileName, content)
}

func multipartRequest(
	t *testing.T, verb, target, fieldName, fileName string, content []byte,
) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	part, err := form.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create the form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write the form file: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close the form: %v", err)
	}
	request := httptest.NewRequest(verb, target, body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Authorization", "Bearer token")
	return request
}

func TestUploadingAnElementAnswers201WithIt(t *testing.T) {
	service := newFakeIngest()
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response, upload(t, "file", "holiday.png", []byte("\x89PNG\r\n\x1a\nbody")))

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", response.Code, response.Body.String())
	}
	if service.calls != 1 {
		t.Fatalf("the service saw %d uploads, want 1", service.calls)
	}
	if service.lastPost != "abcdefgh" {
		t.Errorf("post = %q", service.lastPost)
	}
	// The file name reaches the service because the element's title is built from it.
	if service.lastName != "holiday.png" {
		t.Errorf("file name = %q", service.lastName)
	}
	if string(service.lastBody) != "\x89PNG\r\n\x1a\nbody" {
		t.Errorf("bytes = %q", service.lastBody)
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data["id"] != float64(5) || envelope.Data["type"] != "image" {
		t.Errorf("data = %v", envelope.Data)
	}
}

func urlRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost,
		"/api/v1/account/posts/abcdefgh/elements/urls", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token")
	return request
}

// 201 when every URL worked.
func TestABatchWithNoFailuresIs201(t *testing.T) {
	service := newFakeIngest()
	service.batch = ingest.BatchResult{Added: []ingest.Stored{{ID: 5, Title: "a", Type: "image"}}}
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response, urlRequest(`{"urls":"https://example.test/a.png"}`))

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", response.Code, response.Body.String())
	}
	if service.lastList != "https://example.test/a.png" {
		t.Errorf("list = %q", service.lastList)
	}
}

/*
207 WHEN SOME OF IT WORKED, WHICH IS THE ORDINARY CASE.

An author pasting thirty links will have one that is dead or private. A 201 would say
everything worked and a 422 would say nothing did; both are wrong, and either would leave
the client unable to tell the author which link to look at.
*/
func TestAPartlySuccessfulBatchIs207WithTheFailuresNamed(t *testing.T) {
	service := newFakeIngest()
	service.batch = ingest.BatchResult{
		Added: []ingest.Stored{{ID: 5, Title: "a", Type: "image"}},
		Failed: []ingest.FailedURL{
			{URL: "https://dead.test/x", Reason: ingest.ReasonUnavailable},
		},
	}
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response,
		urlRequest(`{"urls":"https://example.test/a.png,https://dead.test/x"}`))

	if response.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207; body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			Added  []map[string]any `json:"added"`
			Failed []struct {
				URL    string `json:"url"`
				Reason string `json:"reason"`
			} `json:"failed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(envelope.Data.Added) != 1 {
		t.Errorf("added = %v", envelope.Data.Added)
	}
	if len(envelope.Data.Failed) != 1 ||
		envelope.Data.Failed[0].URL != "https://dead.test/x" ||
		envelope.Data.Failed[0].Reason != ingest.ReasonUnavailable {
		t.Errorf("failed = %+v", envelope.Data.Failed)
	}
}

func TestABatchRefusedOutrightIs422(t *testing.T) {
	service := newFakeIngest()
	service.urlErr = &ingest.ErrInvalid{
		Fields: map[string][]string{"urls": {ingest.CodeTooMany}},
	}
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response, urlRequest(`{"urls":"a,b,c"}`))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", response.Code)
	}
	if !strings.Contains(response.Body.String(), ingest.CodeTooMany) {
		t.Errorf("body = %s", response.Body.String())
	}
}

func TestAddingURLsToSomeoneElsesPostIs404(t *testing.T) {
	service := newFakeIngest()
	service.urlErr = ingest.ErrPostNotFound
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response, urlRequest(`{"urls":"https://example.test/a.png"}`))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestAddingURLsRequiresABearerToken(t *testing.T) {
	response := httptest.NewRecorder()
	request := urlRequest(`{"urls":"https://example.test/a.png"}`)
	request.Header.Del("Authorization")

	ingestHandler(newFakeIngest()).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestAnUploadWithNoFilePartIs400(t *testing.T) {
	service := newFakeIngest()
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response, upload(t, "not-file", "a.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if service.calls != 0 {
		t.Error("a request with no file part reached the service")
	}
}

// The declared part size is refused before a byte is read, so a 40 MiB upload does not
// have to be pulled through the process to be told no.
func TestAnOversizedUploadIsRefusedAsAFieldError(t *testing.T) {
	service := newFakeIngest()
	response := httptest.NewRecorder()
	oversized := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, ingest.MaxFileBytes)...)

	ingestHandler(service).ServeHTTP(response, upload(t, "file", "big.png", oversized))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	if service.calls != 0 {
		t.Error("an oversized upload reached the service")
	}
}

func TestTheServicesFieldCodesReachTheClient(t *testing.T) {
	cases := map[string]string{
		"unsupported": ingest.CodeUnsupportedMedia,
		"full":        ingest.CodePostFull,
		"limited":     ingest.CodeRateLimited,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			service := newFakeIngest()
			service.err = &ingest.ErrInvalid{Fields: map[string][]string{"file": {code}}}
			response := httptest.NewRecorder()

			ingestHandler(service).ServeHTTP(response,
				upload(t, "file", "a.png", []byte("\x89PNG\r\n\x1a\n")))

			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", response.Code)
			}
			if !strings.Contains(response.Body.String(), code) {
				t.Errorf("body = %s, want it to carry %q", response.Body.String(), code)
			}
		})
	}
}

// Someone else's post answers 404, the same as every other editor endpoint: a 403 would
// confirm the serial exists.
func TestUploadingToSomeoneElsesPostIs404(t *testing.T) {
	service := newFakeIngest()
	service.err = ingest.ErrPostNotFound
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response, upload(t, "file", "a.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", response.Code, response.Body.String())
	}
}

func TestUploadingRequiresABearerToken(t *testing.T) {
	response := httptest.NewRecorder()
	request := upload(t, "file", "a.png", []byte("\x89PNG\r\n\x1a\n"))
	request.Header.Del("Authorization")

	ingestHandler(newFakeIngest()).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

// An api started without the object-store variables cannot take an upload. That is a
// configuration answer, not a fault.
func TestUploadingWithoutAnObjectStoreIs503(t *testing.T) {
	handler := New(Options{
		Environment:  "test",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthVerifier: staticTokenVerifier{identity: auth.Identity{Subject: "42"}},
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, upload(t, "file", "a.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
}

func TestOnlyPostUploadsAnElement(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/account/posts/abcdefgh/elements/uploads", nil)
	request.Header.Set("Authorization", "Bearer token")

	ingestHandler(newFakeIngest()).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
}

const replaceTarget = "/api/v1/account/elements/91/media"

func TestReplacingAnElementsMediaAnswers200WithIt(t *testing.T) {
	service := newFakeIngest()
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response,
		replacement(t, replaceTarget, "holiday.png", []byte("\x89PNG\r\n\x1a\nbody")))

	// 200, not 201: the element already existed and keeps its id.
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if service.replaceCalls != 1 {
		t.Fatalf("the service saw %d replacements, want 1", service.replaceCalls)
	}
	if service.lastElementID != 91 {
		t.Errorf("element = %d, want 91", service.lastElementID)
	}
	if service.lastName != "holiday.png" {
		t.Errorf("file name = %q", service.lastName)
	}
	if string(service.lastBody) != "\x89PNG\r\n\x1a\nbody" {
		t.Errorf("bytes = %q", service.lastBody)
	}
	if service.calls != 0 {
		t.Errorf("a replacement went to Upload %d times", service.calls)
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data["id"] != float64(5) || envelope.Data["source_url"] == "" {
		t.Errorf("data = %v", envelope.Data)
	}
	if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
}

// The element's own endpoint answers 404 for an element that is not the caller's, and this
// one must not answer differently: a 403 would confirm the id exists.
func TestReplacingSomeoneElsesMediaIs404(t *testing.T) {
	service := newFakeIngest()
	service.err = ingest.ErrElementNotFound
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response, replacement(t, replaceTarget, "a.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", response.Code, response.Body.String())
	}
}

func TestReplacingMediaRequiresABearerToken(t *testing.T) {
	service := newFakeIngest()
	response := httptest.NewRecorder()
	request := replacement(t, replaceTarget, "a.png", []byte("\x89PNG\r\n\x1a\n"))
	request.Header.Del("Authorization")

	ingestHandler(service).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if service.replaceCalls != 0 {
		t.Error("the service ran for an anonymous request")
	}
}

func TestAnUnusableElementIDIs404WithoutReachingTheService(t *testing.T) {
	for _, id := range []string{"0", "-1", "abc"} {
		t.Run(id, func(t *testing.T) {
			service := newFakeIngest()
			response := httptest.NewRecorder()

			ingestHandler(service).ServeHTTP(response,
				replacement(t, "/api/v1/account/elements/"+id+"/media", "a.png", []byte("\x89PNG\r\n\x1a\n")))

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", response.Code)
			}
			if service.replaceCalls != 0 {
				t.Error("the service ran on an id that cannot exist")
			}
		})
	}
}

func TestTheServicesFieldCodesReachTheClientOnAReplacement(t *testing.T) {
	service := newFakeIngest()
	service.err = &ingest.ErrInvalid{Fields: map[string][]string{"file": {ingest.CodeRateLimited}}}
	response := httptest.NewRecorder()

	ingestHandler(service).ServeHTTP(response, replacement(t, replaceTarget, "a.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), ingest.CodeRateLimited) {
		t.Errorf("body = %s", response.Body.String())
	}
}

func TestReplacingMediaWithoutAnObjectStoreIs503(t *testing.T) {
	response := httptest.NewRecorder()

	ingestHandler(nil).ServeHTTP(response, replacement(t, replaceTarget, "a.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
}

func TestOnlyPUTReplacesMedia(t *testing.T) {
	for _, verb := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		t.Run(verb, func(t *testing.T) {
			service := newFakeIngest()
			response := httptest.NewRecorder()
			request := httptest.NewRequest(verb, replaceTarget, nil)
			request.Header.Set("Authorization", "Bearer token")

			ingestHandler(service).ServeHTTP(response, request)

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", response.Code)
			}
			if response.Header().Get("Allow") == "" {
				t.Error("no Allow header")
			}
			if service.replaceCalls != 0 {
				t.Error("the service ran on the wrong method")
			}
		})
	}
}

// The element route and the media route are distinct: PUT on the element edits its title,
// PUT on .../media swaps the file.
func TestTheMediaRouteIsNotTheElementRoute(t *testing.T) {
	service := newFakeIngest()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/account/elements/91",
		strings.NewReader(`{"title":"a"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token")

	ingestHandler(service).ServeHTTP(response, request)

	if service.replaceCalls != 0 {
		t.Error("editing an element reached the media replacement")
	}
}
