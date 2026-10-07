package ui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultLoadTimeout = 5 * time.Second

// loadBoard performs the module's unfiltered read: GET {apiBase}/board over
// real HTTP. Any transport failure or non-200 status is a load failure —
// the result is an error, never an empty board standing in for the truth.
func (p *page) loadBoard() (boardResponse, error) {
	return p.loadBoardFilter("", false)
}

// loadBoardFilter extends the same read with the contract's filter keyword,
// mirroring the api handler's handling of the same parameter: PRESENCE
// decides, never the value. An absent filter (filtered false) is the
// shipped plain GET, byte-frozen leg included. A present one travels as
// the single URL-encoded assignee query parameter — verbatim, an empty
// value included, which the contract reads as a present keyword outside
// the roster. A filtered non-200 (the contract's 422 unknown user among
// them) is a load failure exactly like any other: the stated-failure
// surface owns it, never an empty board standing in for the truth.
func (p *page) loadBoardFilter(keyword string, filtered bool) (boardResponse, error) {
	var board boardResponse
	address := p.apiBase + "/board"
	if filtered {
		address += "?" + url.Values{"assignee": []string{keyword}}.Encode()
	}
	resp, err := p.client.Get(address)
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
