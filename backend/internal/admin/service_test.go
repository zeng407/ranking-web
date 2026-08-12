package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"2pick.app/backend/internal/authoring"
)

func intPointer(value int) *int          { return &value }
func stringPointer(value string) *string { return &value }
func boolPointer(value bool) *bool       { return &value }

// The fakes below stand in for the database, the editor, the shared cache and the session
// store. They record what the service asked of them, because most of the rules in this
// package are about which of those calls happen and in what order.

type fakeAuthoring struct {
	post       authoring.Post
	updateErr  error
	deleteErr  error
	lastOwner  int64
	lastSerial string
	lastDraft  authoring.PostDraft
	deleted    int
}

func (fake *fakeAuthoring) Post(_ context.Context, userID int64, serial string) (authoring.Post, error) {
	fake.lastOwner, fake.lastSerial = userID, serial
	return fake.post, nil
}

func (fake *fakeAuthoring) UpdatePost(
	_ context.Context, userID int64, serial string, draft authoring.PostDraft,
) (authoring.Post, error) {
	fake.lastOwner, fake.lastSerial, fake.lastDraft = userID, serial, draft
	return fake.post, fake.updateErr
}

func (fake *fakeAuthoring) DeletePostAsModerator(_ context.Context, userID int64, serial string) error {
	fake.lastOwner, fake.lastSerial = userID, serial
	fake.deleted++
	return fake.deleteErr
}

func (fake *fakeAuthoring) Elements(
	_ context.Context, userID int64, serial string, _ authoring.ElementQuery,
) (authoring.ElementPage, error) {
	fake.lastOwner, fake.lastSerial = userID, serial
	return authoring.ElementPage{}, nil
}

func (fake *fakeAuthoring) EditElement(
	_ context.Context, userID int64, _ int64, _ authoring.ElementEdit,
) (authoring.Element, error) {
	fake.lastOwner = userID
	return authoring.Element{}, nil
}

func (fake *fakeAuthoring) DeleteElement(_ context.Context, userID int64, _ int64) error {
	fake.lastOwner = userID
	return nil
}

type fakeStore struct {
	owner      int64
	ownerErr   error
	roles      []string
	rolesErr   error
	added      []string
	removed    []string
	censored   *bool
	item       CarouselItem
	itemErr    error
	created    CarouselItem
	lastEdit   CarouselEdit
	positions  []CarouselPosition
	reorderErr error
}

func (fake *fakeStore) PostOwner(context.Context, string) (int64, error) {
	return fake.owner, fake.ownerErr
}

func (fake *fakeStore) ElementOwner(context.Context, int64) (int64, error) {
	return fake.owner, fake.ownerErr
}

func (fake *fakeStore) ListPosts(context.Context, int, int) ([]Post, int, error) {
	return []Post{{Serial: "abc"}}, 1, nil
}

func (fake *fakeStore) SetPostCensored(_ context.Context, _ string, censored bool) error {
	fake.censored = &censored
	return nil
}

func (fake *fakeStore) ListUsers(context.Context, string, int, int) ([]User, int, error) {
	return []User{{ID: 1}}, 1, nil
}

func (fake *fakeStore) UserRoles(context.Context, int64) ([]string, error) {
	return fake.roles, fake.rolesErr
}

func (fake *fakeStore) AddRole(_ context.Context, _ int64, slug string) error {
	fake.added = append(fake.added, slug)
	return nil
}

func (fake *fakeStore) RemoveRole(_ context.Context, _ int64, slug string) error {
	fake.removed = append(fake.removed, slug)
	return nil
}

func (fake *fakeStore) CarouselItems(context.Context) ([]CarouselItem, error) {
	return []CarouselItem{fake.item}, nil
}

func (fake *fakeStore) CarouselItem(context.Context, int64) (CarouselItem, error) {
	return fake.item, fake.itemErr
}

func (fake *fakeStore) CreateCarouselItem(_ context.Context, item CarouselItem) (CarouselItem, error) {
	fake.created = item
	return item, nil
}

func (fake *fakeStore) UpdateCarouselItem(
	_ context.Context, _ int64, edit CarouselEdit,
) (CarouselItem, error) {
	fake.lastEdit = edit
	return fake.item, nil
}

func (fake *fakeStore) DeleteCarouselItem(context.Context, int64) error { return nil }

func (fake *fakeStore) ReorderCarouselItems(_ context.Context, positions []CarouselPosition) error {
	fake.positions = positions
	return fake.reorderErr
}

type fakeCaches struct {
	forgottenUsers []int64
	carousels      int
}

func (fake *fakeCaches) ForgetUserRoles(_ context.Context, userID int64) error {
	fake.forgottenUsers = append(fake.forgottenUsers, userID)
	return nil
}

func (fake *fakeCaches) ForgetCarousels(context.Context) error {
	fake.carousels++
	return nil
}

type fakeSessions struct{ revoked []int64 }

func (fake *fakeSessions) RevokeAll(_ context.Context, userID int64) (int64, error) {
	fake.revoked = append(fake.revoked, userID)
	return 1, nil
}

type fakeAnnouncements struct {
	stored Announcement
	found  bool
	writes int
}

func (fake *fakeAnnouncements) Announcement(context.Context) (Announcement, bool, error) {
	return fake.stored, fake.found, nil
}

func (fake *fakeAnnouncements) PutAnnouncement(_ context.Context, announcement Announcement) error {
	fake.stored, fake.found = announcement, true
	fake.writes++
	return nil
}

type fakeVideos struct {
	resolved ResolvedVideo
	err      error
}

func (fake *fakeVideos) Resolve(context.Context, string) (ResolvedVideo, error) {
	return fake.resolved, fake.err
}

type harness struct {
	service       *Service
	authoring     *fakeAuthoring
	store         *fakeStore
	caches        *fakeCaches
	sessions      *fakeSessions
	announcements *fakeAnnouncements
	videos        *fakeVideos
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	parts := &harness{
		authoring:     &fakeAuthoring{},
		store:         &fakeStore{owner: 42},
		caches:        &fakeCaches{},
		sessions:      &fakeSessions{},
		announcements: &fakeAnnouncements{},
		videos: &fakeVideos{resolved: ResolvedVideo{
			Title: "resolved title", ThumbURL: "https://img.example/1.jpg",
			Source: "youtube", ID: "abc123", URL: "https://youtu.be/abc123",
		}},
	}
	service, err := NewService(ServiceOptions{
		Authoring: parts.authoring, Store: parts.store,
		RoleCache: parts.caches, CarouselCache: parts.caches,
		Sessions: parts.sessions, Announcements: parts.announcements,
		Videos: parts.videos,
		Now:    func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	parts.service = service
	return parts
}

func codeFor(t *testing.T, err error, field string) string {
	t.Helper()
	var invalid *ErrInvalid
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want a validation error", err)
	}
	codes := invalid.Fields[field]
	if len(codes) == 0 {
		t.Fatalf("error = %v, want a code on %q", err, field)
	}
	return codes[0]
}

// A moderator's edit is the owner's edit: the owner id comes off the row, so the
// statements underneath keep their `AND user_id = ?` and no wider write exists.
func TestAModeratorsEditActsAsTheOwner(t *testing.T) {
	harness := newHarness(t)
	harness.store.owner = 7

	if _, err := harness.service.UpdatePost(context.Background(), " serial ",
		PostEdit{Draft: authoring.PostDraft{Title: "t"}}); err != nil {
		t.Fatalf("UpdatePost() error = %v", err)
	}
	if harness.authoring.lastOwner != 7 {
		t.Errorf("owner = %d, want the row's owner 7", harness.authoring.lastOwner)
	}
	if harness.authoring.lastSerial != "serial" {
		t.Errorf("serial = %q, want it trimmed", harness.authoring.lastSerial)
	}
}

// The flag is only written when the edit names it, so fixing a title cannot clear a
// censorship the moderator did not mention.
func TestTheCensorshipFlagIsOnlyWrittenWhenTheEditNamesIt(t *testing.T) {
	harness := newHarness(t)

	if _, err := harness.service.UpdatePost(context.Background(), "abc",
		PostEdit{Draft: authoring.PostDraft{Title: "t"}}); err != nil {
		t.Fatalf("UpdatePost() error = %v", err)
	}
	if harness.store.censored != nil {
		t.Fatalf("censored = %v, want no write", *harness.store.censored)
	}

	if _, err := harness.service.UpdatePost(context.Background(), "abc",
		PostEdit{Draft: authoring.PostDraft{Title: "t"}, Censored: boolPointer(true)}); err != nil {
		t.Fatalf("UpdatePost() error = %v", err)
	}
	if harness.store.censored == nil || !*harness.store.censored {
		t.Errorf("censored = %v, want true", harness.store.censored)
	}
}

// A refused draft must not have written the flag either.
func TestARefusedDraftLeavesTheCensorshipFlagAlone(t *testing.T) {
	harness := newHarness(t)
	harness.authoring.updateErr = &authoring.ErrInvalid{
		Fields: authoring.FieldErrors{"title": {"required"}},
	}

	_, err := harness.service.UpdatePost(context.Background(), "abc",
		PostEdit{Censored: boolPointer(true)})
	if code := codeFor(t, err, "title"); code != "required" {
		t.Errorf("code = %q, want required", code)
	}
	if harness.store.censored != nil {
		t.Errorf("censored = %v, want no write", *harness.store.censored)
	}
}

// A row that vanished between the owner lookup and the write is a 404, not a write to
// somebody else's post.
func TestAVanishedPostIsNotFound(t *testing.T) {
	harness := newHarness(t)
	harness.authoring.deleteErr = authoring.ErrPostNotFound

	if err := harness.service.DeletePost(context.Background(), "abc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeletePost() error = %v, want ErrNotFound", err)
	}
}

// The moderator's delete asks for no password — the role at the HTTP boundary is the proof.
func TestDeletingAPostNeedsNoPassword(t *testing.T) {
	harness := newHarness(t)

	if err := harness.service.DeletePost(context.Background(), "abc"); err != nil {
		t.Fatalf("DeletePost() error = %v", err)
	}
	if harness.authoring.deleted != 1 {
		t.Errorf("deletes = %d, want 1", harness.authoring.deleted)
	}
}

// A ban that leaves the sessions alive is not a ban, and Laravel keeps serving a cached
// role list for an hour unless it is told.
func TestBanningWritesTheRoleThenClearsTheCacheAndTheSessions(t *testing.T) {
	harness := newHarness(t)

	if err := harness.service.BanUser(context.Background(), 12); err != nil {
		t.Fatalf("BanUser() error = %v", err)
	}
	if len(harness.store.added) != 1 || harness.store.added[0] != BannedRoleSlug {
		t.Errorf("added roles = %v, want [%s]", harness.store.added, BannedRoleSlug)
	}
	if len(harness.caches.forgottenUsers) != 1 || harness.caches.forgottenUsers[0] != 12 {
		t.Errorf("forgotten role caches = %v, want [12]", harness.caches.forgottenUsers)
	}
	if len(harness.sessions.revoked) != 1 || harness.sessions.revoked[0] != 12 {
		t.Errorf("revoked sessions = %v, want [12]", harness.sessions.revoked)
	}
}

// Self-banning through the UI was unrecoverable in the original: the ban revokes every
// session, so the moderator who did it could not sign in to undo it.
func TestAnAdministratorCannotBeBanned(t *testing.T) {
	harness := newHarness(t)
	harness.store.roles = []string{"user", AdminRoleSlug}

	err := harness.service.BanUser(context.Background(), 1)
	if !errors.Is(err, ErrCannotBanAdministrator) {
		t.Fatalf("BanUser() error = %v, want ErrCannotBanAdministrator", err)
	}
	if len(harness.store.added) != 0 {
		t.Errorf("added roles = %v, want none", harness.store.added)
	}
	if len(harness.sessions.revoked) != 0 {
		t.Errorf("revoked sessions = %v, want none", harness.sessions.revoked)
	}
}

// An account that is gone cannot be banned or unbanned, which the role read is what
// establishes.
func TestBanningAMissingAccountFails(t *testing.T) {
	harness := newHarness(t)
	harness.store.rolesErr = ErrNotFound

	if err := harness.service.BanUser(context.Background(), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("BanUser() error = %v, want ErrNotFound", err)
	}
	if err := harness.service.UnbanUser(context.Background(), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("UnbanUser() error = %v, want ErrNotFound", err)
	}
}

func TestUnbanningRemovesOnlyTheBannedRole(t *testing.T) {
	harness := newHarness(t)

	if err := harness.service.UnbanUser(context.Background(), 3); err != nil {
		t.Fatalf("UnbanUser() error = %v", err)
	}
	if len(harness.store.removed) != 1 || harness.store.removed[0] != BannedRoleSlug {
		t.Errorf("removed roles = %v, want [%s]", harness.store.removed, BannedRoleSlug)
	}
	if len(harness.sessions.revoked) != 0 {
		t.Errorf("revoked sessions = %v, want none: an unban does not restore or drop sessions",
			harness.sessions.revoked)
	}
}

// A slide is created from the resolved video: the title and the still come from the
// lookup when the moderator left them empty, and the stored URL is the canonical one.
func TestCreatingASlideFillsTheBlanksFromTheVideo(t *testing.T) {
	harness := newHarness(t)

	item, err := harness.service.CreateCarouselItem(context.Background(), CarouselDraft{
		Type: CarouselTypeVideo, VideoURL: "https://www.youtube.com/watch?v=abc123",
	})
	if err != nil {
		t.Fatalf("CreateCarouselItem() error = %v", err)
	}
	if item.Title != "resolved title" {
		t.Errorf("title = %q, want the resolved title", item.Title)
	}
	if item.Description != "resolved title" {
		t.Errorf("description = %q, want it defaulted to the title", item.Description)
	}
	if item.ImageURL != "https://img.example/1.jpg" {
		t.Errorf("image url = %q, want the resolved thumbnail", item.ImageURL)
	}
	if item.VideoURL != "https://youtu.be/abc123" {
		t.Errorf("video url = %q, want the canonical url", item.VideoURL)
	}
	if item.VideoSource != "youtube" || item.VideoID != "abc123" {
		t.Errorf("source/id = %q/%q, want youtube/abc123", item.VideoSource, item.VideoID)
	}
	if item.Position != 1 {
		t.Errorf("position = %d, want the front of the carousel", item.Position)
	}
	if !item.Active {
		t.Error("active = false, want the column's default")
	}
	if harness.caches.carousels != 1 {
		t.Errorf("carousel cache clears = %d, want 1", harness.caches.carousels)
	}
}

// A blank panel on the home page is worse than a refused form.
func TestASlideWhoseVideoCannotBeReadIsRefused(t *testing.T) {
	harness := newHarness(t)
	harness.videos.err = errors.New("no such video")

	_, err := harness.service.CreateCarouselItem(context.Background(), CarouselDraft{
		Type: CarouselTypeVideo, VideoURL: "https://example.com/not-a-video",
	})
	if code := codeFor(t, err, "video_url"); code != CodeUnresolvable {
		t.Errorf("code = %q, want %s", code, CodeUnresolvable)
	}
	if harness.store.created.VideoURL != "" {
		t.Errorf("created = %+v, want no insert", harness.store.created)
	}
}

func TestASlideNeedsAVideoURLAndTheVideoType(t *testing.T) {
	harness := newHarness(t)

	_, err := harness.service.CreateCarouselItem(context.Background(), CarouselDraft{})
	if code := codeFor(t, err, "video_url"); code != "required" {
		t.Errorf("video url code = %q, want required", code)
	}
	if code := codeFor(t, err, "type"); code != "required" {
		t.Errorf("type code = %q, want required", code)
	}

	_, err = harness.service.CreateCarouselItem(context.Background(), CarouselDraft{
		Type: "image", VideoURL: "https://youtu.be/abc123",
	})
	if code := codeFor(t, err, "type"); code != "invalid_policy" {
		t.Errorf("type code = %q, want invalid_policy", code)
	}
}

// Moving only the start must not be able to leave a clip that ends before it begins, so
// the check runs against the stored values as well as the submitted ones.
func TestATrimIsCheckedAgainstTheStoredClip(t *testing.T) {
	harness := newHarness(t)
	harness.store.item = CarouselItem{ID: 5, StartSecond: intPointer(10), EndSecond: intPointer(20)}

	_, err := harness.service.UpdateCarouselItem(context.Background(), 5,
		CarouselEdit{StartSecond: intPointer(30)})
	if code := codeFor(t, err, "video_end_second"); code != CodeInvalidRange {
		t.Errorf("code = %q, want %s", code, CodeInvalidRange)
	}

	if _, err := harness.service.UpdateCarouselItem(context.Background(), 5,
		CarouselEdit{StartSecond: intPointer(12)}); err != nil {
		t.Fatalf("UpdateCarouselItem() error = %v", err)
	}
}

func TestEditingASlideTrimsAndLimitsItsText(t *testing.T) {
	harness := newHarness(t)

	if _, err := harness.service.UpdateCarouselItem(context.Background(), 5,
		CarouselEdit{Title: stringPointer("  a title  ")}); err != nil {
		t.Fatalf("UpdateCarouselItem() error = %v", err)
	}
	if got := *harness.store.lastEdit.Title; got != "a title" {
		t.Errorf("title = %q, want it trimmed", got)
	}

	long := make([]rune, MaxCarouselTextRunes+1)
	for index := range long {
		long[index] = 'あ'
	}
	_, err := harness.service.UpdateCarouselItem(context.Background(), 5,
		CarouselEdit{Title: stringPointer(string(long))})
	if code := codeFor(t, err, "title"); code != "too_long" {
		t.Errorf("code = %q, want too_long", code)
	}
}

// The same id twice would mean the last write silently wins, and which one that is
// depends on the order of a JSON array.
func TestAReorderRefusesRepeatedOrNegativeEntries(t *testing.T) {
	harness := newHarness(t)

	err := harness.service.ReorderCarouselItems(context.Background(),
		[]CarouselPosition{{ID: 1, Position: 1}, {ID: 1, Position: 2}})
	if code := codeFor(t, err, "items"); code != CodeDuplicate {
		t.Errorf("code = %q, want %s", code, CodeDuplicate)
	}

	err = harness.service.ReorderCarouselItems(context.Background(),
		[]CarouselPosition{{ID: 1, Position: -1}})
	if code := codeFor(t, err, "items"); code != CodeInvalidRange {
		t.Errorf("code = %q, want %s", code, CodeInvalidRange)
	}

	err = harness.service.ReorderCarouselItems(context.Background(), nil)
	if code := codeFor(t, err, "items"); code != "required" {
		t.Errorf("code = %q, want required", code)
	}
	if harness.store.positions != nil {
		t.Errorf("positions = %v, want no write", harness.store.positions)
	}
}

func TestAReorderClearsTheCarouselCache(t *testing.T) {
	harness := newHarness(t)

	if err := harness.service.ReorderCarouselItems(context.Background(),
		[]CarouselPosition{{ID: 2, Position: 1}, {ID: 1, Position: 2}}); err != nil {
		t.Fatalf("ReorderCarouselItems() error = %v", err)
	}
	if harness.caches.carousels != 1 {
		t.Errorf("carousel cache clears = %d, want 1", harness.caches.carousels)
	}
}

// A failed reorder must not have told the home page its carousel changed.
func TestAFailedReorderLeavesTheCacheAlone(t *testing.T) {
	harness := newHarness(t)
	harness.store.reorderErr = ErrNotFound

	err := harness.service.ReorderCarouselItems(context.Background(),
		[]CarouselPosition{{ID: 99, Position: 1}})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReorderCarouselItems() error = %v, want ErrNotFound", err)
	}
	if harness.caches.carousels != 0 {
		t.Errorf("carousel cache clears = %d, want 0", harness.caches.carousels)
	}
}

// Every announcement gets a fresh id, because the id is what the client's "never show
// again" cookie holds: reusing one would leave the new banner unseen.
func TestEachAnnouncementGetsItsOwnID(t *testing.T) {
	harness := newHarness(t)

	first, err := harness.service.PublishAnnouncement(context.Background(),
		AnnouncementDraft{Content: "  down for maintenance  "})
	if err != nil {
		t.Fatalf("PublishAnnouncement() error = %v", err)
	}
	second, err := harness.service.PublishAnnouncement(context.Background(),
		AnnouncementDraft{Content: "back up"})
	if err != nil {
		t.Fatalf("PublishAnnouncement() error = %v", err)
	}

	if first.ID == "" || first.ID == second.ID {
		t.Errorf("ids = %q and %q, want two different non-empty ids", first.ID, second.ID)
	}
	if first.Content != "down for maintenance" {
		t.Errorf("content = %q, want it trimmed", first.Content)
	}
	if first.KeepMinutes != DefaultAnnouncementMinutes {
		t.Errorf("minutes = %d, want the default %d", first.KeepMinutes, DefaultAnnouncementMinutes)
	}
}

// The cached created_at is read by clients written against Laravel's format, in Laravel's
// timezone.
func TestAnAnnouncementIsStampedInLaravelsFormatAndTimezone(t *testing.T) {
	harness := newHarness(t)

	announcement, err := harness.service.PublishAnnouncement(context.Background(),
		AnnouncementDraft{Content: "hello", Minutes: 5})
	if err != nil {
		t.Fatalf("PublishAnnouncement() error = %v", err)
	}
	want := time.Unix(1_700_000_000, 0).In(AnnouncementLocation).Format("2006-01-02 15:04:05")
	if announcement.CreatedAt != want {
		t.Errorf("created at = %q, want %q", announcement.CreatedAt, want)
	}
	if announcement.KeepFor() != 5*time.Minute {
		t.Errorf("keep for = %v, want 5m", announcement.KeepFor())
	}
}

// Nothing but a replacement or a cache flush takes a banner down, so the lifetime is
// bounded.
func TestAnAnnouncementsLifetimeIsBounded(t *testing.T) {
	harness := newHarness(t)

	for _, minutes := range []int{-1, MaxAnnouncementMinutes + 1} {
		_, err := harness.service.PublishAnnouncement(context.Background(),
			AnnouncementDraft{Content: "hello", Minutes: minutes})
		if code := codeFor(t, err, "minutes"); code != CodeInvalidRange {
			t.Errorf("minutes = %d: code = %q, want %s", minutes, code, CodeInvalidRange)
		}
	}
	if harness.announcements.writes != 0 {
		t.Errorf("writes = %d, want none", harness.announcements.writes)
	}
}

func TestAnEmptyAnnouncementIsRefused(t *testing.T) {
	harness := newHarness(t)

	_, err := harness.service.PublishAnnouncement(context.Background(),
		AnnouncementDraft{Content: "   "})
	if code := codeFor(t, err, "content"); code != "required" {
		t.Errorf("code = %q, want required", code)
	}
}

// Without a shared cache the announcement endpoints answer 503 and every other admin
// endpoint is unaffected.
func TestAnnouncementsWithoutAStoreAreUnavailable(t *testing.T) {
	service, err := NewService(ServiceOptions{
		Authoring: &fakeAuthoring{}, Store: &fakeStore{},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, _, err := service.Announcement(context.Background()); !errors.Is(err, ErrAnnouncementsUnavailable) {
		t.Errorf("Announcement() error = %v, want ErrAnnouncementsUnavailable", err)
	}
	_, err = service.PublishAnnouncement(context.Background(), AnnouncementDraft{Content: "hello"})
	if !errors.Is(err, ErrAnnouncementsUnavailable) {
		t.Errorf("PublishAnnouncement() error = %v, want ErrAnnouncementsUnavailable", err)
	}
	if _, err := service.Posts(context.Background(), 1); err != nil {
		t.Errorf("Posts() error = %v, want the post list to keep working", err)
	}
}

// A ban still lands without the optional dependencies; they make it immediate, not
// possible.
func TestABanWithoutACacheOrSessionStoreStillWritesTheRole(t *testing.T) {
	store := &fakeStore{}
	service, err := NewService(ServiceOptions{Authoring: &fakeAuthoring{}, Store: store})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.BanUser(context.Background(), 4); err != nil {
		t.Fatalf("BanUser() error = %v", err)
	}
	if len(store.added) != 1 || store.added[0] != BannedRoleSlug {
		t.Errorf("added roles = %v, want [%s]", store.added, BannedRoleSlug)
	}
}

func TestTheServiceNeedsAnEditorAndAStore(t *testing.T) {
	if _, err := NewService(ServiceOptions{Store: &fakeStore{}}); err == nil {
		t.Error("NewService() without an editor error = nil, want an error")
	}
	if _, err := NewService(ServiceOptions{Authoring: &fakeAuthoring{}}); err == nil {
		t.Error("NewService() without a store error = nil, want an error")
	}
}

func TestPagesAreClampedToTheFirst(t *testing.T) {
	harness := newHarness(t)

	posts, err := harness.service.Posts(context.Background(), 0)
	if err != nil {
		t.Fatalf("Posts() error = %v", err)
	}
	if posts.Page != 1 || posts.PerPage != PostsPerPage {
		t.Errorf("page/per page = %d/%d, want 1/%d", posts.Page, posts.PerPage, PostsPerPage)
	}

	users, err := harness.service.Users(context.Background(), " ada ", -3)
	if err != nil {
		t.Fatalf("Users() error = %v", err)
	}
	if users.Page != 1 || users.PerPage != UsersPerPage {
		t.Errorf("page/per page = %d/%d, want 1/%d", users.Page, users.PerPage, UsersPerPage)
	}
}
