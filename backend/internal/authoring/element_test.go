package authoring

import (
	"context"
	"strings"
	"testing"
)

func intPointer(value int) *int          { return &value }
func stringPointer(value string) *string { return &value }

func TestElementQueryDefaultsToTheFirstPageAndTheFullSize(t *testing.T) {
	query := ElementQuery{}.Normalized()

	if query.Page != 1 {
		t.Errorf("page = %d, want 1", query.Page)
	}
	if query.PerPage != ElementsPerPage {
		t.Errorf("per page = %d, want %d", query.PerPage, ElementsPerPage)
	}
	if query.SortBy != "id" {
		t.Errorf("sort by = %q, want id", query.SortBy)
	}
}

// The ceiling exists so one request cannot ask for a post's 1,139 elements at once.
func TestElementQueryClampsAnOversizedPage(t *testing.T) {
	query := ElementQuery{PerPage: 5000}.Normalized()

	if query.PerPage != ElementsPerPage {
		t.Errorf("per page = %d, want it clamped to %d", query.PerPage, ElementsPerPage)
	}
}

// THE ORIGINAL'S PAGINATION READ THE PAGE OUT OF per_page. Asking for 50 per page asked
// for page 50, so a caller who sent per_page could never see the first page. The port
// keeps them separate, which is what this pins.
func TestPageAndPerPageAreIndependent(t *testing.T) {
	query := ElementQuery{Page: 2, PerPage: 25}.Normalized()

	if query.Page != 2 || query.PerPage != 25 {
		t.Errorf("page = %d, per page = %d; want 2 and 25", query.Page, query.PerPage)
	}
}

func TestAnUnknownSortFallsBackToID(t *testing.T) {
	// Anything the SQL does not have a column for. The value never reaches a statement —
	// the repository builds the ORDER BY from a fixed set — but normalising here means
	// the fallback is stated once.
	query := ElementQuery{SortBy: "id); DROP TABLE elements--"}.Normalized()

	if query.SortBy != "id" {
		t.Errorf("sort by = %q, want id", query.SortBy)
	}
}

func TestEditElementTrimsAndLimitsTheTitle(t *testing.T) {
	harness := newHarness(t)

	if _, err := harness.service.EditElement(context.Background(), 7, 1,
		ElementEdit{Title: stringPointer("  a title  ")}); err != nil {
		t.Fatalf("EditElement() error = %v", err)
	}
	if got := *harness.elements.lastEdit.Title; got != "a title" {
		t.Errorf("title = %q, want it trimmed", got)
	}

	_, err := harness.service.EditElement(context.Background(), 7, 1,
		ElementEdit{Title: stringPointer(strings.Repeat("あ", MaxElementTitleRunes+1))})
	if code := codeFor(t, err, "title"); code != CodeTooLong {
		t.Errorf("code = %q, want %q", code, CodeTooLong)
	}
}

// A clip that ends before it starts plays as nothing at all: the original stored it and
// the player then showed an element nobody could see.
func TestEditElementRefusesATrimThatCannotBePlayed(t *testing.T) {
	cases := []struct {
		name  string
		edit  ElementEdit
		field string
	}{
		{"end before start", ElementEdit{StartSecond: intPointer(30), EndSecond: intPointer(10)}, "video_end_second"},
		{"end equal to start", ElementEdit{StartSecond: intPointer(30), EndSecond: intPointer(30)}, "video_end_second"},
		{"negative start", ElementEdit{StartSecond: intPointer(-1)}, "video_start_second"},
		{"negative end", ElementEdit{EndSecond: intPointer(-1)}, "video_end_second"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newHarness(t)

			_, err := harness.service.EditElement(context.Background(), 7, 1, testCase.edit)
			if code := codeFor(t, err, testCase.field); code != CodeInvalidRange {
				t.Errorf("code = %q, want %q", code, CodeInvalidRange)
			}
			if harness.elements.lastEdit.StartSecond != nil || harness.elements.lastEdit.EndSecond != nil {
				t.Error("an unplayable trim reached the store")
			}
		})
	}
}

// Only one end of the trim moving is a normal edit: the other keeps whatever it holds.
func TestEditElementAcceptsOneEndOfTheTrim(t *testing.T) {
	harness := newHarness(t)

	if _, err := harness.service.EditElement(context.Background(), 7, 1,
		ElementEdit{StartSecond: intPointer(12)}); err != nil {
		t.Fatalf("EditElement() error = %v", err)
	}
	if got := *harness.elements.lastEdit.StartSecond; got != 12 {
		t.Errorf("start = %d, want 12", got)
	}
	if harness.elements.lastEdit.EndSecond != nil {
		t.Error("an end was written although none was sent")
	}
}

// One refresh per post the element was ranked in, which is what DeleteElementRank
// dispatched — an element can belong to more than one post.
func TestDeletingAnElementRefreshesEveryPostItWasRankedIn(t *testing.T) {
	harness := newHarness(t)
	harness.elements.affected = []int64{11, 22}

	if err := harness.service.DeleteElement(context.Background(), 7, 5); err != nil {
		t.Fatalf("DeleteElement() error = %v", err)
	}
	if len(harness.ranks.posts) != 2 ||
		harness.ranks.posts[0] != 11 || harness.ranks.posts[1] != 22 {
		t.Errorf("refreshed %v, want [11 22]", harness.ranks.posts)
	}
}

func TestDeletingAnElementThatRanksNowhereRefreshesNothing(t *testing.T) {
	harness := newHarness(t)
	harness.elements.affected = nil

	if err := harness.service.DeleteElement(context.Background(), 7, 5); err != nil {
		t.Fatalf("DeleteElement() error = %v", err)
	}
	if len(harness.ranks.posts) != 0 {
		t.Errorf("refreshed %v, want nothing", harness.ranks.posts)
	}
}
