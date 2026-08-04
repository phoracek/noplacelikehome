package wedos

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	OK = 1000
)

// parseResponse decodes a WAPI response envelope. A non-OK (non-1000) result
// code is returned as an error so callers don't silently treat a rejected
// request (auth failure, invalid TTL, commit rejection, ...) as success.
//
// NOTE: upstream only errored on a JSON decode failure and returned the
// envelope with no error for any result code, which made failed commits/adds
// look like successes. See NOTICE.
func (p *Provider) parseResponse(response *http.Response, into any) (*responseEnvelope, error) {
	var envelope responseEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return nil, err
	}

	if envelope.Response.Code != OK {
		return &envelope, fmt.Errorf("wedos: command %q failed: %s (code %d)",
			envelope.Response.Command, envelope.Response.Result, envelope.Response.Code)
	}

	if envelope.Response.Data != nil && into != nil {
		if err := json.Unmarshal(envelope.Response.Data, into); err != nil {
			return nil, err
		}
	}

	return &envelope, nil
}
