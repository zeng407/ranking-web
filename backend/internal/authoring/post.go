package authoring

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// The author's side of a post: the list, the editor's metadata, and deletion. From
// Api\MyPostController and PostService.
//
// Separate from internal/publicpost, which serves the same rows to visitors. The two
// have almost nothing in common beyond the table: one is a cached public read path, this
// one writes and is never cached.

// Limits from config/setting.php.
const (
	MaxTitleRunes       = 50
	MaxDescriptionRunes = 300
	MaxTags             = 5
	MaxTagNameRunes     = 15
	// MaxElements is post_max_element_count. Enforced when elements are added, which is
	// why one production post holds 1,139 of them: the cap has never applied to reading.
	MaxElements = 1024
	// SerialLength matches SerialGenerator::genPostSerial. All 6,201 production serials
	// are exactly this long.
	SerialLength = 8
	// ElementsPerPage is MyPostController::ELEMENTS_PER_PAGE, used as both the default
	// and the ceiling.
	ElementsPerPage = 100
	// PostsPerPage is Laravel's default paginate() size, which the post list used.
	PostsPerPage = 15
)

// Access policies, matching the post_policies.access_policy enum.
const (
	PolicyPrivate  = "private"
	PolicyPublic   = "public"
	PolicyPassword = "password"
)

// Post is the author's view of one of their own posts.
//
// Mirrors MyPostResource, with one addition: HasPassword. The Blade editor could read
// post_policy.password server-side to decide whether to offer "change the password" or
// "set one"; a client cannot, and the hash itself must never leave the server.
type Post struct {
	Serial            string
	Title             string
	Description       string
	AccessPolicy      string
	HasPassword       bool
	Tags              []string
	PlayCount         int
	ThisWeekPlayCount int
	LastWeekPlayCount int
	CreatedAt         time.Time
}

// PostDraft is what create and update accept.
type PostDraft struct {
	Title        string
	Description  string
	AccessPolicy string
	// Password is the plaintext, or "" for "leave whatever is stored". Hashed with
	// SHA-256 before it reaches the database — see hashPostPassword.
	Password string
	// Tags replaces the post's tags. Nil means "leave them alone", which is how the
	// original behaved: the rule is `sometimes`, and syncTags only ran on update.
	Tags []string
}

// Validation codes, extending the ones the auth package already defines so one client
// renderer covers every form.
const (
	CodeRequired      = "required"
	CodeTooLong       = "too_long"
	CodeInvalidPolicy = "invalid_policy"
	CodeTooMany       = "too_many"
	CodeIncorrect     = "incorrect"
)

// FieldErrors maps a form field to machine-readable reasons.
type FieldErrors map[string][]string

// ErrInvalid carries the per-field reasons a draft was refused.
type ErrInvalid struct {
	Fields FieldErrors
}

func (err *ErrInvalid) Error() string {
	return fmt.Sprintf("authoring: rejected: %v", err.Fields)
}

func invalid(field, code string) error {
	return &ErrInvalid{Fields: FieldErrors{field: []string{code}}}
}

// ErrPostNotFound means no post with that serial belongs to the caller.
//
// The same error for "does not exist" and "belongs to someone else", deliberately:
// telling the difference would turn the endpoint into an oracle for which serials exist.
var ErrPostNotFound = errors.New("authoring: post not found")

// ErrElementNotFound means the element does not exist, or none of the posts it belongs
// to are the caller's. Same reasoning as ErrPostNotFound.
var ErrElementNotFound = errors.New("authoring: element not found")

// PostStore is the persistence this package needs.
type PostStore interface {
	// ListPosts reads one page of the user's posts, newest first, with the total.
	ListPosts(ctx context.Context, userID int64, page, perPage int) ([]Post, int, error)
	// Post reads one post the user owns. ErrPostNotFound when there is none.
	Post(ctx context.Context, userID int64, serial string) (Post, error)
	// CreatePost writes the post and its policy in one transaction and answers with the
	// serial it was given.
	CreatePost(ctx context.Context, userID int64, serial string, draft PostDraft, passwordHash string) error
	// UpdatePost writes the metadata, the policy, and — when draft.Tags is non-nil — the
	// tags. passwordHash of nil leaves the stored password untouched.
	UpdatePost(ctx context.Context, userID int64, serial string, draft PostDraft, passwordHash *string) error
	// DeletePost soft-deletes the post and its elements, detaches its tags and removes
	// its rank reports. It answers with the post's id so the caller can queue the
	// recomputation the original queued.
	DeletePost(ctx context.Context, userID int64, serial string) (int64, error)
}

// PasswordChecker verifies the account password that deletion asks for.
type PasswordChecker interface {
	// PasswordHash reads the account's bcrypt hash, which is "" for the accounts that
	// signed in through Google.
	PasswordHash(ctx context.Context, userID int64) (string, error)
}

// RankRefresher queues the work a deletion invalidates.
type RankRefresher interface {
	RefreshPostRank(ctx context.Context, postID int64) error
}

// Service holds the rules.
type Service struct {
	posts     PostStore
	elements  ElementStore
	passwords PasswordChecker
	ranks     RankRefresher
}

type ServiceOptions struct {
	Posts     PostStore
	Elements  ElementStore
	Passwords PasswordChecker
	// Ranks is optional. Without it a deletion still happens and the stale report is
	// left for the daily schedule to correct, which is better than refusing the delete.
	Ranks RankRefresher
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Posts == nil {
		return nil, errors.New("authoring: post store is required")
	}
	if options.Elements == nil {
		return nil, errors.New("authoring: element store is required")
	}
	if options.Passwords == nil {
		return nil, errors.New("authoring: password checker is required")
	}
	return &Service{
		posts:     options.Posts,
		elements:  options.Elements,
		passwords: options.Passwords,
		ranks:     options.Ranks,
	}, nil
}

// Posts lists the caller's posts.
func (service *Service) Posts(ctx context.Context, userID int64, page int) ([]Post, int, error) {
	if page < 1 {
		page = 1
	}
	return service.posts.ListPosts(ctx, userID, page, PostsPerPage)
}

// Post reads one of the caller's posts.
func (service *Service) Post(ctx context.Context, userID int64, serial string) (Post, error) {
	return service.posts.Post(ctx, userID, serial)
}

// CreatePost writes a new post and answers with its serial.
func (service *Service) CreatePost(ctx context.Context, userID int64, draft PostDraft) (string, error) {
	draft.Title = strings.TrimSpace(draft.Title)
	draft.Description = strings.TrimSpace(draft.Description)

	fields := FieldErrors{}
	validateTitle(draft.Title, fields)
	validateDescription(draft.Description, fields)
	validatePolicy(draft.AccessPolicy, fields)
	// A password post needs one on creation: there is no stored password to fall back on.
	if draft.AccessPolicy == PolicyPassword && draft.Password == "" {
		fields["password"] = []string{CodeRequired}
	}
	validateTags(draft.Tags, fields)
	if len(fields) > 0 {
		return "", &ErrInvalid{Fields: fields}
	}

	serial, err := NewPostSerial()
	if err != nil {
		return "", err
	}
	if err := service.posts.CreatePost(
		ctx, userID, serial, draft, hashPostPassword(draft.Password)); err != nil {
		return "", err
	}
	return serial, nil
}

// UpdatePost rewrites a post's metadata, policy and tags.
//
// THE PASSWORD RULES ARE THE ORIGINAL'S, from validatePostPassword. A post that becomes
// or stays password-protected keeps its stored password when none is submitted, so
// editing a title does not silently clear the password. A post that leaves the password
// policy has its password erased, so switching back to `password` later cannot revive an
// old one the author has forgotten.
func (service *Service) UpdatePost(
	ctx context.Context, userID int64, serial string, draft PostDraft,
) (Post, error) {
	draft.Title = strings.TrimSpace(draft.Title)
	draft.Description = strings.TrimSpace(draft.Description)

	fields := FieldErrors{}
	validateTitle(draft.Title, fields)
	validateDescription(draft.Description, fields)
	validatePolicy(draft.AccessPolicy, fields)
	validateTags(draft.Tags, fields)
	if len(fields) > 0 {
		return Post{}, &ErrInvalid{Fields: fields}
	}

	existing, err := service.posts.Post(ctx, userID, serial)
	if err != nil {
		return Post{}, err
	}

	var passwordHash *string
	switch {
	case draft.AccessPolicy != PolicyPassword:
		// Cleared, not left behind.
		cleared := ""
		passwordHash = &cleared
	case draft.Password != "":
		hashed := hashPostPassword(draft.Password)
		passwordHash = &hashed
	case !existing.HasPassword:
		// Becoming password-protected with nothing to protect it with.
		return Post{}, invalid("password", CodeRequired)
	}
	// The remaining case — password policy, no submission, a password already stored —
	// leaves passwordHash nil, which the store reads as "do not touch it".

	if err := service.posts.UpdatePost(ctx, userID, serial, draft, passwordHash); err != nil {
		return Post{}, err
	}
	return service.posts.Post(ctx, userID, serial)
}

// DeletePost removes a post after the caller proves who they are.
//
// THE ACCOUNT PASSWORD IS ASKED FOR, BUT ONLY WHEN THERE IS ONE. The original ran
// password_verify against users.password unconditionally, and that column is an empty
// string for the 11,040 accounts created through Google — password_verify against an
// empty hash is always false, so none of those accounts could ever delete a post. Here an
// account with no password deletes on the strength of its access token alone, which is
// the same proof every other write on this endpoint rests on. Accounts that do have a
// password still have to type it.
func (service *Service) DeletePost(
	ctx context.Context, userID int64, serial, accountPassword string,
) error {
	hash, err := service.passwords.PasswordHash(ctx, userID)
	if err != nil {
		return err
	}
	if hash != "" {
		if accountPassword == "" {
			return invalid("password", CodeRequired)
		}
		if len(accountPassword) > 72 {
			// bcrypt reads 72 bytes; anything past that would compare against a
			// truncation the user never typed.
			accountPassword = accountPassword[:72]
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(accountPassword)); err != nil {
			return invalid("password", CodeIncorrect)
		}
	}

	postID, err := service.posts.DeletePost(ctx, userID, serial)
	if err != nil {
		return err
	}
	service.refreshRank(ctx, postID)
	return nil
}

// DeletePostAsModerator removes somebody else's post without asking for a password.
//
// The account password proves the caller is the owner, and a moderator is not: they have
// no owner's password to type, and their own would prove nothing about this post. The
// authorization is the admin role, checked at the HTTP boundary before the request ever
// reaches here — see internal/admin. Everything else about the deletion is identical,
// which is the reason this lives beside DeletePost instead of being reimplemented over
// there: one cascade, one rank refresh.
func (service *Service) DeletePostAsModerator(
	ctx context.Context, userID int64, serial string,
) error {
	postID, err := service.posts.DeletePost(ctx, userID, serial)
	if err != nil {
		return err
	}
	service.refreshRank(ctx, postID)
	return nil
}

// refreshRank queues the recomputation a deletion invalidates.
//
// Failures are swallowed on purpose: the rows are already gone, and refusing the request
// now would be a lie. The daily rank schedule corrects a missed one.
func (service *Service) refreshRank(ctx context.Context, postID int64) {
	if service.ranks == nil || postID == 0 {
		return
	}
	_ = service.ranks.RefreshPostRank(ctx, postID)
}

func validateTitle(title string, fields FieldErrors) {
	switch {
	case title == "":
		fields["title"] = []string{CodeRequired}
	case utf8.RuneCountInString(title) > MaxTitleRunes:
		fields["title"] = []string{CodeTooLong}
	}
}

func validateDescription(description string, fields FieldErrors) {
	switch {
	case description == "":
		fields["description"] = []string{CodeRequired}
	case utf8.RuneCountInString(description) > MaxDescriptionRunes:
		fields["description"] = []string{CodeTooLong}
	}
}

func validatePolicy(policy string, fields FieldErrors) {
	switch policy {
	case PolicyPrivate, PolicyPublic, PolicyPassword:
	case "":
		fields["access_policy"] = []string{CodeRequired}
	default:
		fields["access_policy"] = []string{CodeInvalidPolicy}
	}
}

func validateTags(tags []string, fields FieldErrors) {
	if tags == nil {
		return
	}
	if len(tags) > MaxTags {
		fields["tags"] = []string{CodeTooMany}
		return
	}
	for _, tag := range tags {
		if utf8.RuneCountInString(strings.TrimSpace(tag)) > MaxTagNameRunes {
			fields["tags"] = []string{CodeTooLong}
			return
		}
	}
}

// CleanTags trims, drops empties and removes duplicates.
//
// The original attached tags in a loop with no duplicate check, so a post could hold the
// same tag twice — the pivot has a composite primary key, which turns that into a
// constraint error rather than a duplicate row, and the whole update would fail on a
// user's typo.
func CleanTags(tags []string) []string {
	cleaned := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, already := seen[tag]; already {
			continue
		}
		seen[tag] = struct{}{}
		cleaned = append(cleaned, tag)
	}
	return cleaned
}

// hashPostPassword hashes a post password.
//
// SHA-256, NOT BCRYPT, because that is what the column holds: PostPolicy compares
// hash('sha256', $password) against post_policies.password, and the game and rank pages
// still read it through Laravel. A stronger hash here would lock 1,035 password-protected
// posts out of their own visitors.
//
// It is the right choice for its own reasons too: this is a shared door code handed out
// with the link, not a credential, and every visitor's request has to check it.
func hashPostPassword(password string) string {
	if password == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

const serialAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// NewPostSerial generates a post serial.
//
// Eight lowercase alphanumerics, matching SerialGenerator::genPostSerial — which built
// them with Str::random and then lower-cased, so the alphabet is the same. The PHP
// retried on collision; this does not, for the same reason the room serials do not:
// 36^8 is 2.8e12 against 6,201 posts, and the unique index on serial is the real
// guarantee either way.
func NewPostSerial() (string, error) {
	raw := make([]byte, SerialLength)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("authoring: read random bytes: %w", err)
	}
	serial := make([]byte, SerialLength)
	for index, value := range raw {
		serial[index] = serialAlphabet[int(value)%len(serialAlphabet)]
	}
	return string(serial), nil
}
