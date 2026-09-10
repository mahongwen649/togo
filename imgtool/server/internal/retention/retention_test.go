package retention

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"imgtool/server/internal/history"
)

func TestCleanerDeletesExpiredObjectsBeforeHistoryRecords(t *testing.T) {
	store := &fakeHistoryStore{
		batches: [][]history.ExpiredRecord{{
			{
				UserID:    "alice",
				HistoryID: "his_old",
				Files: []history.File{
					{ObjectKey: "aiImg/reference.png"},
					{ObjectKey: "aiImg/result.png"},
				},
			},
		}, {}},
	}
	deleter := &fakeObjectDeleter{}
	cleaner := Cleaner{
		History:   store,
		Deleter:   deleter,
		Retention: 15 * 24 * time.Hour,
	}

	result, err := cleaner.RunOnce(time.UnixMilli(20 * int64(24*time.Hour/time.Millisecond)))
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if result.RecordsDeleted != 1 || result.ObjectsDeleted != 2 {
		t.Fatalf("result = %#v", result)
	}
	if store.cutoffs[0] != int64(5*24*time.Hour/time.Millisecond) {
		t.Fatalf("cutoff = %d", store.cutoffs[0])
	}
	if !reflect.DeepEqual(deleter.deleted, []string{"aiImg/reference.png", "aiImg/result.png"}) {
		t.Fatalf("deleted objects = %#v", deleter.deleted)
	}
	if !reflect.DeepEqual(store.deleted, []string{"alice/his_old"}) {
		t.Fatalf("deleted histories = %#v", store.deleted)
	}
}

func TestCleanerKeepsHistoryWhenObjectDeletionFails(t *testing.T) {
	store := &fakeHistoryStore{
		batches: [][]history.ExpiredRecord{{
			{
				UserID:    "alice",
				HistoryID: "his_old",
				Files:     []history.File{{ObjectKey: "aiImg/result.png"}},
			},
		}},
	}
	deleter := &fakeObjectDeleter{err: errors.New("delete failed")}
	cleaner := Cleaner{History: store, Deleter: deleter}

	if _, err := cleaner.RunOnce(time.Now()); err == nil {
		t.Fatal("RunOnce succeeded after object deletion failed")
	}
	if len(store.deleted) != 0 {
		t.Fatalf("history was deleted despite object delete failure: %#v", store.deleted)
	}
}

type fakeHistoryStore struct {
	batches [][]history.ExpiredRecord
	cutoffs []int64
	deleted []string
}

func (s *fakeHistoryStore) ExpiredHistory(cutoffMs int64, _ int) ([]history.ExpiredRecord, error) {
	s.cutoffs = append(s.cutoffs, cutoffMs)
	if len(s.batches) == 0 {
		return nil, nil
	}
	batch := s.batches[0]
	s.batches = s.batches[1:]
	return batch, nil
}

func (s *fakeHistoryStore) DeleteHistory(userID string, historyID string) error {
	s.deleted = append(s.deleted, userID+"/"+historyID)
	return nil
}

type fakeObjectDeleter struct {
	deleted []string
	err     error
}

func (d *fakeObjectDeleter) DeleteObject(objectKey string) error {
	if d.err != nil {
		return d.err
	}
	d.deleted = append(d.deleted, objectKey)
	return nil
}
