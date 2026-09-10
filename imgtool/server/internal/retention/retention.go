package retention

import (
	"context"
	"time"

	"imgtool/server/internal/history"
)

const DefaultRetention = 15 * 24 * time.Hour

type HistoryStore interface {
	ExpiredHistory(cutoffMs int64, limit int) ([]history.ExpiredRecord, error)
	DeleteHistory(userID string, historyID string) error
}

type ObjectDeleter interface {
	DeleteObject(objectKey string) error
}

type Cleaner struct {
	History   HistoryStore
	Deleter   ObjectDeleter
	Retention time.Duration
	BatchSize int
}

type Result struct {
	RecordsDeleted int
	ObjectsDeleted int
}

func (c Cleaner) RunOnce(now time.Time) (Result, error) {
	retention := c.Retention
	if retention <= 0 {
		retention = DefaultRetention
	}
	batchSize := c.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	if c.History == nil {
		return Result{}, nil
	}
	cutoffMs := now.Add(-retention).UnixMilli()
	result := Result{}
	for {
		expired, err := c.History.ExpiredHistory(cutoffMs, batchSize)
		if err != nil {
			return result, err
		}
		if len(expired) == 0 {
			return result, nil
		}
		for _, record := range expired {
			for _, file := range record.Files {
				if c.Deleter != nil {
					if err := c.Deleter.DeleteObject(file.ObjectKey); err != nil {
						return result, err
					}
				}
				result.ObjectsDeleted++
			}
			if err := c.History.DeleteHistory(record.UserID, record.HistoryID); err != nil {
				return result, err
			}
			result.RecordsDeleted++
		}
		if len(expired) < batchSize {
			return result, nil
		}
	}
}

func (c Cleaner) Start(ctx context.Context, interval time.Duration, logError func(error)) {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	go func() {
		if _, err := c.RunOnce(time.Now()); err != nil && logError != nil {
			logError(err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if _, err := c.RunOnce(now); err != nil && logError != nil {
					logError(err)
				}
			}
		}
	}()
}
