package admin

import (
	"context"
	"errors"
	"strings"

	"2pick.app/backend/internal/authoring"
)

// Post is the moderator's row in the post list.
//
// The author's own view (authoring.Post) is what the editor shows; this adds the two
// things only a moderator needs — who owns it, and whether it has been censored — and
// leaves out the weekly play counts, which the list does not draw.
type Post struct {
	Serial       string
	Title        string
	Description  string
	AccessPolicy string
	Censored     bool
	PlayCount    int
	OwnerID      int64
	OwnerName    string
	OwnerEmail   string
	CreatedAt    string
}

// PostPage is one page of the site-wide post list.
type PostPage struct {
	Posts   []Post
	Total   int
	Page    int
	PerPage int
}

// PostEdit is what a moderator may change about a post.
//
// The authoring half (title, description, policy, password, tags) is the same form the
// author sees, and is validated by the same code. Censored is the moderation-only field,
// and nil means "leave it": the Blade form posted the checkbox on every save, but a
// client that only wants to fix a title should not have to know the flag's current value
// to avoid clearing it.
type PostEdit struct {
	Draft    authoring.PostDraft
	Censored *bool
}

// Posts lists every post on the site, newest first.
func (service *Service) Posts(ctx context.Context, page int) (PostPage, error) {
	if page < 1 {
		page = 1
	}
	posts, total, err := service.store.ListPosts(ctx, page, PostsPerPage)
	if err != nil {
		return PostPage{}, wrap("list posts", err)
	}
	return PostPage{Posts: posts, Total: total, Page: page, PerPage: PostsPerPage}, nil
}

// Post reads one post as its author would see it, whoever the author is.
func (service *Service) Post(ctx context.Context, serial string) (authoring.Post, error) {
	ownerID, err := service.store.PostOwner(ctx, strings.TrimSpace(serial))
	if err != nil {
		return authoring.Post{}, wrap("resolve the post owner", err)
	}
	post, err := service.authoring.Post(ctx, ownerID, serial)
	return post, translateAuthoringError(err)
}

// UpdatePost rewrites a post on its author's behalf, and sets or clears the censorship
// flag when the edit names it.
//
// The two writes are not one transaction, deliberately: they are independent columns on
// the same row, and the metadata write is the one that can be refused for a bad draft. A
// failure between them leaves the post edited and the flag as it was, which the moderator
// sees in the response they get back.
func (service *Service) UpdatePost(
	ctx context.Context, serial string, edit PostEdit,
) (authoring.Post, error) {
	serial = strings.TrimSpace(serial)
	ownerID, err := service.store.PostOwner(ctx, serial)
	if err != nil {
		return authoring.Post{}, wrap("resolve the post owner", err)
	}

	post, err := service.authoring.UpdatePost(ctx, ownerID, serial, edit.Draft)
	if err != nil {
		return authoring.Post{}, translateAuthoringError(err)
	}
	if edit.Censored != nil {
		if err := service.store.SetPostCensored(ctx, serial, *edit.Censored); err != nil {
			return authoring.Post{}, wrap("set the censorship flag", err)
		}
	}
	return post, nil
}

// DeletePost removes a post without asking for anybody's password.
//
// The author's own delete asks for their account password; a moderator has no way to know
// it, and the original admin route asked for nothing either. The proof here is the admin
// role on the access token, checked at the HTTP boundary.
func (service *Service) DeletePost(ctx context.Context, serial string) error {
	serial = strings.TrimSpace(serial)
	ownerID, err := service.store.PostOwner(ctx, serial)
	if err != nil {
		return wrap("resolve the post owner", err)
	}
	return translateAuthoringError(service.authoring.DeletePostAsModerator(ctx, ownerID, serial))
}

// Elements lists one page of a post's elements.
func (service *Service) Elements(
	ctx context.Context, serial string, query authoring.ElementQuery,
) (authoring.ElementPage, error) {
	ownerID, err := service.store.PostOwner(ctx, strings.TrimSpace(serial))
	if err != nil {
		return authoring.ElementPage{}, wrap("resolve the post owner", err)
	}
	page, err := service.authoring.Elements(ctx, ownerID, serial, query)
	return page, translateAuthoringError(err)
}

// EditElement changes an element's title or a video's trim points.
func (service *Service) EditElement(
	ctx context.Context, elementID int64, edit authoring.ElementEdit,
) (authoring.Element, error) {
	ownerID, err := service.store.ElementOwner(ctx, elementID)
	if err != nil {
		return authoring.Element{}, wrap("resolve the element owner", err)
	}
	element, err := service.authoring.EditElement(ctx, ownerID, elementID, edit)
	return element, translateAuthoringError(err)
}

// DeleteElement removes an element from the post it belongs to.
func (service *Service) DeleteElement(ctx context.Context, elementID int64) error {
	ownerID, err := service.store.ElementOwner(ctx, elementID)
	if err != nil {
		return wrap("resolve the element owner", err)
	}
	return translateAuthoringError(service.authoring.DeleteElement(ctx, ownerID, elementID))
}

// translateAuthoringError maps the authoring package's "not yours" errors onto this
// package's plain ErrNotFound.
//
// A moderator's request cannot fail for lack of ownership — the owner is whoever the row
// says — so the only way those errors surface here is a row that disappeared between the
// lookup and the call, which is a 404 either way. Validation errors pass through
// untouched: they are the same form errors the author would get.
func translateAuthoringError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, authoring.ErrPostNotFound), errors.Is(err, authoring.ErrElementNotFound):
		return ErrNotFound
	default:
		var fields *authoring.ErrInvalid
		if errors.As(err, &fields) {
			return err
		}
		return wrap("edit as a moderator", err)
	}
}
