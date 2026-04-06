package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized")

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type UserSession struct {
	Chain     string    `json:"chain"`
	Status    string    `json:"status"`
	Position  *int      `json:"positionInWaitList"`
	ClassData ClassData `json:"classData"`
}

type ClassData struct {
	ID          string       `json:"id"`
	StartTime   time.Time    `json:"startTime"`
	EndTime     time.Time    `json:"endTime"`
	Location    Location     `json:"location"`
	Activity    Activity     `json:"activity"`
	Instructors []Instructor `json:"instructors"`
}

type Location struct {
	ID     string  `json:"id"`
	Studio string  `json:"studio"`
	Room   *string `json:"room"`
}

type Activity struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Color *string `json:"color"`
}

type Instructor struct {
	Name string `json:"name"`
}

type ClassDetail struct {
	ID               string         `json:"id"`
	StartTime        time.Time      `json:"startTime"`
	EndTime          time.Time      `json:"endTime"`
	Location         Location       `json:"location"`
	Activity         DetailActivity `json:"activity"`
	Instructors      []Instructor   `json:"instructors"`
	IsCancelled      bool           `json:"isCancelled"`
	CancelText       *string        `json:"cancelText"`
	TotalSlots       *int           `json:"totalSlots"`
	AvailableSlots   *int           `json:"availableSlots"`
	WaitingListCount *int           `json:"waitingListCount"`
}

type DetailActivity struct {
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	Description           *string `json:"description"`
	AdditionalInformation *string `json:"additionalInformation"`
	Color                 *string `json:"color"`
}

func (c *Client) GetUserSessions(token string) ([]UserSession, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/user/sessions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var sessions []UserSession
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (c *Client) GetClassDetail(token, chain, classID string) (*ClassDetail, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/classes/"+chain+"/"+classID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var detail ClassDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *Client) CancelBooking(token, chain, classID string) error {
	body, err := json.Marshal(map[string]string{"classId": classID})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/"+chain+"/cancel-booking", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cancellation failed: %d", resp.StatusCode)
	}
	return nil
}
