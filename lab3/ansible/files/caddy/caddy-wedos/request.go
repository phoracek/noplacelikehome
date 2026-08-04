package wedos

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	BaseURL        = "https://api.wedos.com/wapi/json"
	PragueTimezone = "Europe/Prague"
	TimezoneOffset = 1 * 60 * 60
)

// getCurrentTime returns current time in a specified timezone.
// Offset is used if the timezone cannot be automatically determined.
func getCurrentTime(timezone string, offset int) time.Time {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.FixedZone(timezone, offset)
	}

	return time.Now().In(location)
}

// createAuthString returns an authentification string used for API requests.
func (p *Provider) createAuthString() string {
	timeInPrague := getCurrentTime(PragueTimezone, TimezoneOffset)
	hour := timeInPrague.Hour()

	passwordHash := sha1.Sum([]byte(p.Password))
	passwordHashString := hex.EncodeToString(passwordHash[:])

	// WEDOS expects the hour zero-padded to two digits (00-23, Europe/Prague).
	// Upstream used %d, which drops the leading zero for hours 0-9 and makes
	// auth fail every day from 00:00-09:59. See NOTICE.
	concat := fmt.Sprintf("%s%s%02d", p.Username, passwordHashString, hour)
	authHash := sha1.Sum([]byte(concat))

	return hex.EncodeToString(authHash[:])
}

// buildRequest creates an HTTP POST request for the Wedos API with the specified command, transaction ID, and payload.
func (p *Provider) buildRequest(ctx context.Context, command string, clTRID string, payload any) (req *http.Request, err error) {
	if p.Username == "" || p.Password == "" {
		err = fmt.Errorf("buildRequest: missing username and/or password")
		return
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, BaseURL, nil)
	if err != nil {
		err = fmt.Errorf("buildRequest: failed to create req: %v", err)
		return
	}

	requestBody := request{
		User:    p.Username,
		Auth:    p.createAuthString(),
		Command: command,
		ClTRID:  clTRID,
	}

	if payload != nil {
		rawPayload, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("buildRequest: failed to marshal payload: %v", err)
		}
		requestBody.Data = rawPayload
	}

	requestEnvelope := requestEnvelope{
		Request: requestBody,
	}

	requestJSON, err := json.Marshal(requestEnvelope)
	if err != nil {
		return nil, fmt.Errorf("buildRequest: failed to marshal req: %v", err)
	}

	requestWrapped := map[string]string{
		"request": string(requestJSON),
	}

	form := url.Values{}
	for k, v := range requestWrapped {
		form.Set(k, v)
	}

	requestEncoded := strings.NewReader(form.Encode())

	req.Body = io.NopCloser(requestEncoded)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req, nil
}

// doRequest performs an HTTP request using the provided http.Request and returns the response or an error.
func (p *Provider) doRequest(request *http.Request) (response *http.Response, err error) {
	var client *http.Client
	if p.httpClient != nil {
		client = p.httpClient
	} else {
		client = &http.Client{}
	}

	response, err = client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("doRequest: failed to do request: %v", err)
	}

	return response, nil
}
