package ui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

const defaultLoadTimeout = 5 * time.Second

// listTodos performs the module's one read: GET {apiBase}/todos over real
// HTTP. Any transport failure or non-200 status is a load failure — the
// result is an error, never an empty list standing in for the truth.
func (p *page) listTodos() ([]todo, error) {
	resp, err := p.client.Get(p.apiBase + "/todos")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("api returned non-200 for todo list")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var todos []todo
	if err := json.Unmarshal(body, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}
