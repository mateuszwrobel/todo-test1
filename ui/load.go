package ui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

const defaultLoadTimeout = 5 * time.Second

// loadBoard performs the module's one read: GET {apiBase}/board over real
// HTTP. Any transport failure or non-200 status is a load failure — the
// result is an error, never an empty board standing in for the truth.
func (p *page) loadBoard() (boardResponse, error) {
	var board boardResponse
	resp, err := p.client.Get(p.apiBase + "/board")
	if err != nil {
		return board, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return board, errors.New("api returned non-200 for board read")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return board, err
	}
	if err := json.Unmarshal(body, &board); err != nil {
		return board, err
	}
	return board, nil
}
