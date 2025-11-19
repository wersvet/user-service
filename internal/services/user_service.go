package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type UserService struct {
	client  *http.Client
	baseURL string
}

type UserDTO struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func NewUserService(baseURL string) *UserService {
	return &UserService{
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: baseURL,
	}
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*UserDTO, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/auth/user/%d", s.baseURL, id), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth service returned status %d", resp.StatusCode)
	}

	var user UserDTO
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}
