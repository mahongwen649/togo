package channels

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("channel not found")

type Store interface {
	Create(channel Channel) (Channel, error)
	Update(channel Channel) (Channel, error)
	Delete(userID string, id string) error
	List(userID string) ([]Channel, error)
	ByID(userID string, id string) (Channel, error)
}

type MemoryStore struct {
	mu       sync.RWMutex
	channels map[string]Channel
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{channels: map[string]Channel{}}
}

func (s *MemoryStore) Create(channel Channel) (Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[channel.ID] = cloneChannel(channel)
	return cloneChannel(channel), nil
}

func (s *MemoryStore) Update(channel Channel) (Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.channels[channel.ID]
	if !ok || existing.UserID != channel.UserID {
		return Channel{}, ErrNotFound
	}
	s.channels[channel.ID] = cloneChannel(channel)
	return cloneChannel(channel), nil
}

func (s *MemoryStore) Delete(userID string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.channels[id]
	if !ok || existing.UserID != userID {
		return ErrNotFound
	}
	delete(s.channels, id)
	return nil
}

func (s *MemoryStore) List(userID string) ([]Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channels := []Channel{}
	for _, channel := range s.channels {
		if channel.UserID == userID {
			channels = append(channels, cloneChannel(channel))
		}
	}
	return channels, nil
}

func (s *MemoryStore) ByID(userID string, id string) (Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channel, ok := s.channels[id]
	if !ok || channel.UserID != userID {
		return Channel{}, ErrNotFound
	}
	return cloneChannel(channel), nil
}

func cloneChannel(channel Channel) Channel {
	channel.Models = append([]ModelEntry(nil), channel.Models...)
	return channel
}
