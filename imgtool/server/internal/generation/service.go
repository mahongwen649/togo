package generation

import (
	"errors"
	"log"
	"strings"
	"time"

	"imgtool/server/internal/channels"
	"imgtool/server/internal/history"
)

var ErrInvalidRequest = errors.New("invalid generation request")
var ErrStorageNotConfigured = errors.New("image storage is not configured")

type TextRequest struct {
	ChannelID string
	ModelID   string
	Prompt    string
	Image     []byte
	MimeType  string
}

type TextResult struct {
	TaskID     string `json:"taskId"`
	HistoryID  string `json:"historyId"`
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
	ResultText string `json:"resultText"`
}

type ImageRequest struct {
	ChannelID   string
	ModelID     string
	Prompt      string
	Size        string
	Quality     string
	AspectRatio string
	Resolution  string
	Count       int
	Capability  string
	Image       []byte
	MimeType    string
}

type ImageResult struct {
	Bytes    []byte
	MimeType string
}

type StoreImageInput struct {
	UserID      string
	ChannelID   string
	ChannelSlug string
	ModelID     string
	ModelSlug   string
	TaskID      string
	Role        string
	Index       int
	Bytes       []byte
	MimeType    string
}

type StoredImage struct {
	StorageProvider string
	Bucket          string
	ObjectKey       string
	Role            string
	MediaType       string
	MimeType        string
	SizeBytes       int64
}

type ImageResultResponse struct {
	TaskID     string         `json:"taskId"`
	HistoryID  string         `json:"historyId"`
	Status     string         `json:"status"`
	DurationMs int64          `json:"durationMs"`
	Files      []history.File `json:"files"`
}

type ImageTaskResponse struct {
	TaskID     string `json:"taskId"`
	HistoryID  string `json:"historyId"`
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
}

type Provider interface {
	GenerateText(snapshot channels.Snapshot, request TextRequest) (string, error)
	GenerateImage(snapshot channels.Snapshot, request ImageRequest) ([]ImageResult, error)
}

type Storage interface {
	SaveGeneratedImage(input StoreImageInput) (StoredImage, error)
}

type Service struct {
	channels *channels.Service
	history  *history.Service
	provider Provider
	storage  Storage
}

func NewService(channelService *channels.Service, historyService *history.Service, provider Provider, storage ...Storage) *Service {
	service := &Service{channels: channelService, history: historyService, provider: provider}
	if len(storage) > 0 {
		service.storage = storage[0]
	}
	return service
}

func (s *Service) GenerateText(userID string, request TextRequest) (TextResult, error) {
	if request.ChannelID == "" || request.ModelID == "" || request.Prompt == "" || len(request.Image) == 0 {
		return TextResult{}, ErrInvalidRequest
	}
	snapshot, err := s.channels.ResolveSnapshot(userID, request.ChannelID, request.ModelID, "image-to-text")
	if err != nil {
		return TextResult{}, err
	}
	task, err := s.history.CreateTask(userID, history.TaskInput{
		Kind:        "text",
		ChannelID:   request.ChannelID,
		ChannelName: snapshot.ChannelName,
		BaseURL:     snapshot.BaseURL,
		ModelID:     snapshot.ModelID,
		Capability:  snapshot.Capability,
		Prompt:      request.Prompt,
		Parameters:  map[string]any{},
	})
	if err != nil {
		return TextResult{}, err
	}
	text, err := s.provider.GenerateText(snapshot, request)
	if err != nil {
		_, _ = s.history.CompleteTask(userID, task.ID, history.CompleteInput{
			Status:        "失败",
			ErrorSource:   "upstream",
			FailureReason: err.Error(),
		})
		return TextResult{}, err
	}
	record, err := s.history.CompleteTask(userID, task.ID, history.CompleteInput{
		Status:     "完成",
		ResultText: text,
	})
	if err != nil {
		return TextResult{}, err
	}
	return TextResult{
		TaskID:     task.ID,
		HistoryID:  record.ID,
		Status:     record.Status,
		DurationMs: record.DurationMs,
		ResultText: record.ResultText,
	}, nil
}

func (s *Service) GenerateImage(userID string, request ImageRequest) (ImageResultResponse, error) {
	task, snapshot, request, err := s.createImageTask(userID, request)
	if err != nil {
		return ImageResultResponse{}, err
	}
	record, err := s.completeImageTask(userID, task, snapshot, request)
	if err != nil {
		return ImageResultResponse{}, err
	}
	return ImageResultResponse{
		TaskID:     task.ID,
		HistoryID:  record.ID,
		Status:     record.Status,
		DurationMs: record.DurationMs,
		Files:      record.Files,
	}, nil
}

func (s *Service) StartImage(userID string, request ImageRequest) (ImageTaskResponse, error) {
	task, snapshot, request, err := s.createImageTask(userID, request)
	if err != nil {
		return ImageTaskResponse{}, err
	}
	go func() {
		if _, err := s.completeImageTask(userID, task, snapshot, request); err != nil {
			log.Printf("generation stage=background task=%s status=failed error=%s", task.ID, err)
		}
	}()
	return ImageTaskResponse{
		TaskID: task.ID,
		Status: task.Status,
	}, nil
}

func (s *Service) createImageTask(userID string, request ImageRequest) (history.Task, channels.Snapshot, ImageRequest, error) {
	if request.ChannelID == "" || request.ModelID == "" || strings.TrimSpace(request.Prompt) == "" {
		return history.Task{}, channels.Snapshot{}, ImageRequest{}, ErrInvalidRequest
	}
	request.Count = 1
	if s.storage == nil {
		return history.Task{}, channels.Snapshot{}, ImageRequest{}, ErrStorageNotConfigured
	}
	capability := request.Capability
	if capability == "" {
		capability = "text-to-image"
	}
	snapshot, err := s.channels.ResolveSnapshot(userID, request.ChannelID, request.ModelID, capability)
	if err != nil {
		return history.Task{}, channels.Snapshot{}, ImageRequest{}, err
	}
	task, err := s.history.CreateTask(userID, history.TaskInput{
		Kind:        "image",
		ChannelID:   request.ChannelID,
		ChannelName: snapshot.ChannelName,
		BaseURL:     snapshot.BaseURL,
		ModelID:     snapshot.ModelID,
		Capability:  snapshot.Capability,
		Prompt:      request.Prompt,
		Parameters: imageTaskParameters(request),
	})
	if err != nil {
		return history.Task{}, channels.Snapshot{}, ImageRequest{}, err
	}
	return task, snapshot, request, nil
}


func imageTaskParameters(request ImageRequest) map[string]any {
	params := map[string]any{"count": request.Count}
	if request.Size != "" {
		params["size"] = request.Size
	}
	if request.Quality != "" {
		params["quality"] = request.Quality
	}
	if request.AspectRatio != "" {
		params["aspectRatio"] = request.AspectRatio
	}
	if request.Resolution != "" {
		params["resolution"] = request.Resolution
	}
	return params
}

func (s *Service) completeImageTask(userID string, task history.Task, snapshot channels.Snapshot, request ImageRequest) (history.Record, error) {
	providerStarted := time.Now()
	images, err := s.provider.GenerateImage(snapshot, request)
	log.Printf(
		"generation stage=provider task=%s channel=%s model=%s capability=%s count=%d duration_ms=%d images=%d",
		task.ID,
		snapshot.ChannelName,
		snapshot.ModelID,
		snapshot.Capability,
		request.Count,
		time.Since(providerStarted).Milliseconds(),
		len(images),
	)
	if err != nil {
		_, _ = s.history.CompleteTask(userID, task.ID, history.CompleteInput{
			Status:        "失败",
			ErrorSource:   "upstream",
			FailureReason: err.Error(),
		})
		return history.Record{}, err
	}
	files := make([]history.FileInput, 0, len(images)+1)
	if len(request.Image) > 0 {
		storageStarted := time.Now()
		stored, err := s.storage.SaveGeneratedImage(StoreImageInput{
			UserID:      userID,
			ChannelID:   request.ChannelID,
			ChannelSlug: snapshot.ChannelName,
			ModelID:     snapshot.ModelID,
			ModelSlug:   snapshot.ModelID,
			TaskID:      task.ID,
			Role:        "reference",
			Index:       0,
			Bytes:       request.Image,
			MimeType:    request.MimeType,
		})
		log.Printf(
			"generation stage=storage_reference task=%s bytes=%d mime=%s duration_ms=%d",
			task.ID,
			len(request.Image),
			request.MimeType,
			time.Since(storageStarted).Milliseconds(),
		)
		if err != nil {
			_, _ = s.history.CompleteTask(userID, task.ID, history.CompleteInput{
				Status:        "失败",
				ErrorSource:   "storage",
				FailureReason: err.Error(),
			})
			return history.Record{}, err
		}
		files = append(files, history.FileInput{
			StorageProvider: stored.StorageProvider,
			Bucket:          stored.Bucket,
			ObjectKey:       stored.ObjectKey,
			Role:            "reference",
			MediaType:       stored.MediaType,
			MimeType:        stored.MimeType,
			SizeBytes:       stored.SizeBytes,
		})
	}
	for index, image := range images {
		storageStarted := time.Now()
		stored, err := s.storage.SaveGeneratedImage(StoreImageInput{
			UserID:      userID,
			ChannelID:   request.ChannelID,
			ChannelSlug: snapshot.ChannelName,
			ModelID:     snapshot.ModelID,
			ModelSlug:   snapshot.ModelID,
			TaskID:      task.ID,
			Role:        "result",
			Index:       index,
			Bytes:       image.Bytes,
			MimeType:    image.MimeType,
		})
		log.Printf(
			"generation stage=storage task=%s index=%d bytes=%d mime=%s duration_ms=%d",
			task.ID,
			index,
			len(image.Bytes),
			image.MimeType,
			time.Since(storageStarted).Milliseconds(),
		)
		if err != nil {
			_, _ = s.history.CompleteTask(userID, task.ID, history.CompleteInput{
				Status:        "失败",
				ErrorSource:   "storage",
				FailureReason: err.Error(),
			})
			return history.Record{}, err
		}
		files = append(files, history.FileInput{
			StorageProvider: stored.StorageProvider,
			Bucket:          stored.Bucket,
			ObjectKey:       stored.ObjectKey,
			Role:            "result",
			MediaType:       stored.MediaType,
			MimeType:        stored.MimeType,
			SizeBytes:       stored.SizeBytes,
		})
	}
	record, err := s.history.CompleteTask(userID, task.ID, history.CompleteInput{
		Status: "完成",
		Files:  files,
	})
	if err != nil {
		return history.Record{}, err
	}
	return record, nil
}
