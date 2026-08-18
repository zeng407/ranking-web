package gameplay

import (
	"database/sql"
	"testing"
)

func TestMatchesForStage(t *testing.T) {
	tests := []struct {
		stage  int
		remain int
		want   int
	}{
		{stage: 1, remain: 45, want: 23},
		{stage: 1, remain: 300, want: 150},
		{stage: 2, remain: 150, want: 22},
		{stage: 2, remain: 128, want: 64},
		{stage: 3, remain: 128, want: 64},
	}
	for _, test := range tests {
		if got := matchesForStage(test.stage, test.remain); got != test.want {
			t.Errorf("matchesForStage(%d, %d) = %d, want %d", test.stage, test.remain, got, test.want)
		}
	}
}

func TestBatchPositionContinuesIncompleteStage(t *testing.T) {
	stage, remain, index, total := batchPosition([]persistedRound{
		{CurrentRound: 1, OfRound: 4, RemainElements: 7},
		{CurrentRound: 2, OfRound: 4, RemainElements: 6},
	}, 8)
	if stage != 1 || remain != 6 || index != 2 || total != 4 {
		t.Fatalf("position = (%d,%d,%d,%d)", stage, remain, index, total)
	}
}

func TestUsablePreviewDropsPlaceholderElements(t *testing.T) {
	id := int64(7)
	url := "https://file.2pick.app/low/267x400/example.webp"

	if usablePreview(nil) != nil {
		t.Error("a missing element should not produce a preview")
	}
	// The legacy payload emits an all-null element when a post has fewer than
	// two renderable options; rendering it would show a broken card.
	if usablePreview(&PreviewElement{}) != nil {
		t.Error("an element without an id should not produce a preview")
	}
	if usablePreview(&PreviewElement{ID: &id}) != nil {
		t.Error("an element without any image URL should not produce a preview")
	}

	element := usablePreview(&PreviewElement{ID: &id, URL: &url, Previewable: true})
	if element == nil || element.URL == nil || *element.URL != url {
		t.Fatalf("expected the preview to be kept, got %+v", element)
	}
}

func TestUUIDFormat(t *testing.T) {
	identifier, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	if len(identifier) != 36 || identifier[14] != '4' || identifier[8] != '-' || identifier[13] != '-' {
		t.Fatalf("unexpected UUID %q", identifier)
	}
}

func TestTopResultElementIDsUseChampionThenFinalNineEliminations(t *testing.T) {
	rounds := []persistedRound{
		{RemainElements: 6, WinnerID: 2, LoserID: 6},
		{RemainElements: 1, WinnerID: 1, LoserID: 2},
		{RemainElements: 3, WinnerID: 3, LoserID: 4},
		{RemainElements: 2, WinnerID: 1, LoserID: 3},
		{RemainElements: 5, WinnerID: 5, LoserID: 7},
		{RemainElements: 4, WinnerID: 4, LoserID: 5},
	}

	got := topResultElementIDs(rounds, 10)
	want := []int64{1, 2, 3, 4, 5, 7, 6}
	if len(got) != len(want) {
		t.Fatalf("ids = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("ids = %#v, want %#v", got, want)
		}
	}
}

// The four schema snapshots this has to survive. The hidden-only case is the one that
// used to fail: deleted_at was named in the query itself, so a database without it
// answered "Unknown column 'rr.deleted_at'" for every finished game.
func TestResultRankVisibilitySuitsEverySchemaSnapshot(t *testing.T) {
	tests := []struct {
		hasDeletedAt, hasHidden bool
		want                    string
	}{
		{false, false, ""},
		{true, false, " AND rr.deleted_at IS NULL"},
		{false, true, " AND rr.hidden = 0"},
		{true, true, " AND rr.deleted_at IS NULL AND rr.hidden = 0"},
	}
	for _, test := range tests {
		if got := resultRankVisibility(test.hasDeletedAt, test.hasHidden); got != test.want {
			t.Errorf("resultRankVisibility(%t, %t) = %q, want %q",
				test.hasDeletedAt, test.hasHidden, got, test.want)
		}
	}
}

func TestPositiveRankPointerHidesZeroAndMissingRanks(t *testing.T) {
	if positiveRankPointer(sql.NullInt64{}) != nil {
		t.Fatal("a missing rank must remain absent")
	}
	if positiveRankPointer(sql.NullInt64{Int64: 0, Valid: true}) != nil {
		t.Fatal("rank zero must remain absent")
	}
	rank := positiveRankPointer(sql.NullInt64{Int64: 12, Valid: true})
	if rank == nil || *rank != 12 {
		t.Fatalf("positive rank = %#v", rank)
	}
}

func TestFinalPairAsDisplayedKeepsTheSideThePlayerSaw(t *testing.T) {
	final := Vote{WinnerID: 7, LoserID: 9}
	tests := []struct {
		name       string
		candidates []int64
		want       string
		wantOK     bool
	}{
		{name: "winner on the left", candidates: []int64{7, 9}, want: "7,9", wantOK: true},
		{name: "winner on the right", candidates: []int64{9, 7}, want: "9,7", wantOK: true},
		{name: "missing", candidates: nil, wantOK: false},
		{name: "not a pair", candidates: []int64{7, 9, 11}, wantOK: false},
		{name: "a stranger in the pair", candidates: []int64{7, 11}, wantOK: false},
		{name: "same element twice", candidates: []int64{7, 7}, wantOK: false},
	}
	for _, test := range tests {
		got, ok := finalPairAsDisplayed(test.candidates, final)
		if ok != test.wantOK {
			t.Errorf("%s: ok = %t, want %t", test.name, ok, test.wantOK)
			continue
		}
		if ok && got != test.want {
			t.Errorf("%s: candidates = %q, want %q", test.name, got, test.want)
		}
	}
}
