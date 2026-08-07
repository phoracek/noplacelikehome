package wedos

import "encoding/json"

type request struct {
	User    string          `json:"user"`
	Auth    string          `json:"auth"`
	Command string          `json:"command"`
	ClTRID  string          `json:"clTRID"`
	Test    string          `json:"test,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type requestEnvelope struct {
	Request request `json:"request"`
}

type response struct {
	Code      int             `json:"code"`
	Result    string          `json:"result"`
	Timestamp uint            `json:"timestamp"`
	ClTRID    string          `json:"clTRID"`
	SvTRID    string          `json:"svTRID"`
	Command   string          `json:"command"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type responseEnvelope struct {
	Response response `json:"response"`
}

type dnsRowsListResponse struct {
	Row row `json:"row"`
}

type row []rowItem

type rowItem struct {
	ID            string `json:"ID"`
	Name          string `json:"name"`
	TTL           string `json:"ttl"`
	Rdtype        string `json:"rdtype"`
	Rdata         string `json:"rdata"`
	ChangedDate   string `json:"changed_date"`
	AuthorComment string `json:"author_comment"`
}

type dnsAppendResponse struct {
	Domain string `json:"domain"`
	ID     string `json:"row_id"`
}

type recordKey struct {
	Name string
	Type string
	Data string
}
