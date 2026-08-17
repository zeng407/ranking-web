package publiccontent

import (
	"context"
	"errors"

	"2pick.app/backend/internal/postaccess"
)

var ErrNotFound = errors.New("public resource not found")

type Repository interface {
	Tags(ctx context.Context, keyword string, limit int) ([]Tag, error)
	HotTags(ctx context.Context, postLimit int) (map[string]int64, error)
	CarouselItems(ctx context.Context) ([]CarouselItem, error)
	Posts(ctx context.Context, query PostsQuery) (PostsPage, error)
	Champions(ctx context.Context, limit int) ([]Champion, error)
	// The rank methods take a caller because a post's ranks are as protected as the
	// post: Laravel had a separate private pair of endpoints behind
	// PostPolicy::readRank for exactly these three reads.
	Ranks(ctx context.Context, postSerial string, group RankGroup, page, perPage int, caller postaccess.Caller) (RanksPage, error)
	SearchRanks(ctx context.Context, postSerial, keyword string, limit int, caller postaccess.Caller) ([]RankReport, error)
	Rank(ctx context.Context, postSerial string, elementID int64, ranges []string, caller postaccess.Caller) (RankDetails, error)
}

type RankGroup string

const (
	RankGroupCumulative RankGroup = "cumulative"
	RankGroupRecent1000 RankGroup = "recent_1000"
)

func (group RankGroup) Valid() bool {
	return group == RankGroupCumulative || group == RankGroupRecent1000
}

type Tag struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type CarouselItem struct {
	Title            *string `json:"title"`
	Description      *string `json:"description"`
	ImageURL         *string `json:"image_url"`
	VideoURL         *string `json:"video_url"`
	Position         int     `json:"position"`
	Type             string  `json:"type"`
	VideoSource      *string `json:"video_source"`
	VideoID          *string `json:"video_id"`
	VideoStartSecond *string `json:"video_start_second"`
}

type PostElement struct {
	ID          *int64  `json:"id"`
	URL         *string `json:"url"`
	URL2        *string `json:"url2"`
	Title       *string `json:"title"`
	Type        *string `json:"type"`
	VideoSource *string `json:"video_source"`
	Previewable bool    `json:"previewable"`
}

type Post struct {
	Title         string      `json:"title"`
	Serial        string      `json:"serial"`
	IsPrivate     bool        `json:"is_private"`
	Description   string      `json:"description"`
	Element1      PostElement `json:"element1"`
	Element2      PostElement `json:"element2"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	PlayCount     int64       `json:"play_count"`
	ElementsCount int64       `json:"elements_count"`
	Tags          []string    `json:"tags"`
	IsCensored    int         `json:"is_censored"`
}

type PostsQuery struct {
	Sort    string
	Range   string
	Keyword string
	Page    int
	PerPage int
}

type PostsPage struct {
	Items      []Post `json:"items"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	Total      int64  `json:"total"`
	TotalPages int    `json:"total_pages"`
}

type ChampionElement struct {
	Name     string  `json:"name"`
	ThumbURL *string `json:"thumb_url"`
	IsWinner bool    `json:"is_winner"`
}

type Champion struct {
	PostTitle  string           `json:"post_title"`
	PostSerial string           `json:"post_serial"`
	Left       *ChampionElement `json:"left"`
	Right      *ChampionElement `json:"right"`
	DateTime   string           `json:"datetime"`
	ThumbURL   *string          `json:"thumb_url"`
	Key        string           `json:"key"`
}

type RankElement struct {
	Title          *string `json:"title"`
	Type           string  `json:"type"`
	ID             int64   `json:"id"`
	VideoID        *string `json:"video_id"`
	SourceURL      *string `json:"source_url"`
	VideoSource    *string `json:"video_source"`
	ThumbURL       *string `json:"thumb_url"`
	LowThumbURL    *string `json:"lowthumb_url"`
	MediumThumbURL *string `json:"mediumthumb_url"`
}

// RankSnapshot is one element's standing in a past ranking run.
type RankSnapshot struct {
	Rank    int64  `json:"rank"`
	WinRate string `json:"win_rate"`
	Date    string `json:"date"`
}

type RankReport struct {
	Rank    *int64      `json:"rank"`
	WinRate string      `json:"win_rate"`
	Date    string      `json:"date"`
	Element RankElement `json:"element"`
	// Recent is the same element's place in the latest thousand-vote snapshot, so
	// a cumulative listing can show both standings on one row. Absent when the
	// snapshot did not place this element, and on the thousand-vote listing
	// itself, where it would only repeat the row.
	Recent *RankSnapshot `json:"recent,omitempty"`
}

type RanksPage struct {
	Items      []RankReport `json:"items"`
	Group      RankGroup    `json:"group"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	Total      int64        `json:"total"`
	TotalPages int          `json:"total_pages"`
}

type RankGroups struct {
	Cumulative *RankReport `json:"cumulative"`
	Recent1000 *RankReport `json:"recent_1000"`
}

type RankHistory struct {
	Rank    int64  `json:"rank"`
	WinRate string `json:"win_rate"`
	Date    string `json:"date"`
}

type RankDetails struct {
	Current *RankReport              `json:"current"`
	Groups  RankGroups               `json:"groups"`
	History map[string][]RankHistory `json:"history"`
}
