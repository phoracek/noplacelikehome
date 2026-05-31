// Package wedos implements a DNS record management client compatible
// with the libdns interfaces for WEDOS DNS.
package wedos

import (
	"context"
	"net/http"
	"strings"

	"github.com/libdns/libdns"
)

const (
	GetRecords    = "dns-rows-list"
	AppendRecords = "dns-row-add"
	DeleteRecords = "dns-row-delete"
	UpdateRecords = "dns-row-update"
	CommitDomain  = "dns-domain-commit"
)

// commitDomain publishes staged changes for the zone. WEDOS DNS modifications
// (dns-row-add/update/delete) only stage changes; they do not become live until
// the zone is committed with dns-domain-commit. Upstream omits this, so records
// were never published. Added locally — see NOTICE.
func (p *Provider) commitDomain(ctx context.Context, zone string) error {
	payload := map[string]string{
		"name": strings.TrimSuffix(zone, "."),
	}

	request, err := p.buildRequest(ctx, CommitDomain, "CommitDomain", payload)
	if err != nil {
		return err
	}

	response, err := p.doRequest(request)
	if err != nil {
		return err
	}

	_, err = p.parseResponse(response, nil)
	return err
}

// Provider implements libdns for Wedos using the WAPI hour‑based SHA‑1 token.
type Provider struct {
	Username   string
	Password   string
	httpClient *http.Client
}

func NewProvider(username, password string) *Provider {
	return &Provider{
		Username:   username,
		Password:   password,
		httpClient: &http.Client{},
	}
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	payload := map[string]string{
		"domain": zone,
	}

	request, err := p.buildRequest(ctx, GetRecords, "GetRecords", payload)
	if err != nil {
		return nil, err
	}

	response, err := p.doRequest(request)
	if err != nil {
		return nil, err
	}

	var data dnsRowsListResponse
	_, err = p.parseResponse(response, &data)
	if err != nil {
		return nil, err
	}

	var records []libdns.Record
	for _, row := range data.Row {
		record, err := toLibDNSRecord(row)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	var addedRecords []libdns.Record

	for _, record := range records {
		payload, err := toWedosDNSRecord(record, zone)
		if err != nil {
			return nil, err
		}

		request, err := p.buildRequest(ctx, AppendRecords, "AppendRecords", payload)
		if err != nil {
			return nil, err
		}

		response, err := p.doRequest(request)
		if err != nil {
			return nil, err
		}

		var data dnsAppendResponse
		if _, err := p.parseResponse(response, &data); err != nil {
			return nil, err
		}

		addedRecords = append(addedRecords, record)
	}

	if len(addedRecords) > 0 {
		if err := p.commitDomain(ctx, zone); err != nil {
			return nil, err
		}
	}

	return addedRecords, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	payload := map[string]string{
		"domain": zone,
	}

	request, err := p.buildRequest(ctx, GetRecords, "GetRecords", payload)
	if err != nil {
		return nil, err
	}
	response, err := p.doRequest(request)
	if err != nil {
		return nil, err
	}
	var remoteData dnsRowsListResponse
	_, err = p.parseResponse(response, &remoteData)
	if err != nil {
		return nil, err
	}

	remoteMap := make(map[recordKey]rowItem)
	for _, remoteRecord := range remoteData.Row {
		key := recordKey{
			Name: remoteRecord.Name,
			Type: remoteRecord.Rdtype,
			Data: "",
		}
		remoteMap[key] = remoteRecord
	}

	var updatedRecords []libdns.Record
	for _, recordToSet := range records {
		wedosRecordToSet, err := toWedosDNSRecord(recordToSet, zone)
		if err != nil {
			return nil, err
		}

		recordToSetKey := recordKey{
			Name: wedosRecordToSet["name"],
			Type: wedosRecordToSet["type"],
			Data: "",
		}

		_, ok := remoteMap[recordToSetKey]
		if !ok {
			payload := wedosRecordToSet
			request, err := p.buildRequest(ctx, AppendRecords, "AppendRecords", payload)
			if err != nil {
				return nil, err
			}

			response, err := p.doRequest(request)
			if err != nil {
				return nil, err
			}

			var data dnsAppendResponse
			if _, err := p.parseResponse(response, &data); err != nil {
				return nil, err
			}

			updatedRecords = append(updatedRecords, recordToSet)
		} else {
			payload := map[string]string{
				"domain": zone,
				"row_id": remoteMap[recordToSetKey].ID,
				"ttl":    wedosRecordToSet["ttl"],
				"rdata":  wedosRecordToSet["rdata"],
			}

			request, err := p.buildRequest(ctx, UpdateRecords, "UpdateRecords", payload)
			if err != nil {
				return nil, err
			}

			response, err := p.doRequest(request)
			if err != nil {
				return nil, err
			}

			_, err = p.parseResponse(response, nil)
			if err != nil {
				return nil, err
			}

			updatedRecords = append(updatedRecords, recordToSet)
		}
	}

	if len(updatedRecords) > 0 {
		if err := p.commitDomain(ctx, zone); err != nil {
			return nil, err
		}
	}

	return updatedRecords, nil
}

// DeleteRecords deletes the specified records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	payload := map[string]string{
		"domain": zone,
	}

	request, err := p.buildRequest(ctx, GetRecords, "GetRecords", payload)
	if err != nil {
		return nil, err
	}

	response, err := p.doRequest(request)
	if err != nil {
		return nil, err
	}

	var data dnsRowsListResponse
	_, err = p.parseResponse(response, &data)
	if err != nil {
		return nil, err
	}

	// Match on name+type only and delete EVERY matching row. certmagic's cleanup
	// record value can differ in representation from what WEDOS stores, and for
	// ACME _acme-challenge TXT records removing all values at the name is the
	// desired, reliable behaviour — it also clears any orphan tokens left by an
	// interrupted earlier attempt. Upstream matched on rdata too and removed each
	// key after a single hit, so stale challenge records piled up and broke
	// Let's Encrypt validation. See NOTICE.
	type nameType struct{ Name, Type string }
	wanted := make(map[nameType]libdns.Record, len(records))
	for _, rec := range records {
		wedosRec, err := toWedosDNSRecord(rec, zone)
		if err != nil {
			return nil, err
		}

		wanted[nameType{wedosRec["name"], wedosRec["type"]}] = rec
	}

	var deletedRecords []libdns.Record
	for _, row := range data.Row {
		rec, ok := wanted[nameType{row.Name, row.Rdtype}]
		if !ok {
			continue
		}

		payload := map[string]string{
			"domain": zone,
			"row_id": row.ID,
		}

		req, err := p.buildRequest(ctx, DeleteRecords, "DeleteRecords", payload)
		if err != nil {
			return nil, err
		}

		resp, err := p.doRequest(req)
		if err != nil {
			return nil, err
		}

		if _, err := p.parseResponse(resp, nil); err != nil {
			return nil, err
		}

		deletedRecords = append(deletedRecords, rec)
	}

	if len(deletedRecords) > 0 {
		if err := p.commitDomain(ctx, zone); err != nil {
			return nil, err
		}
	}

	return deletedRecords, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
