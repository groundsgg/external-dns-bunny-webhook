package bunny

import (
	"context"
	"sync"
)

type mockCall struct {
	method string
	args   any
}

type mockClient struct {
	mu sync.Mutex

	listResp  *ListZonesResponse
	listErr   error
	createOK  *Record
	createErr error
	updateErr error
	deleteErr error

	calls []mockCall
}

func (m *mockClient) record(method string, args any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockCall{method, args})
}

func (m *mockClient) Calls() []mockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mockCall, len(m.calls))
	copy(out, m.calls)
	return out
}

func (m *mockClient) CountByMethod(method string) int {
	n := 0
	for _, c := range m.Calls() {
		if c.method == method {
			n++
		}
	}
	return n
}

func (m *mockClient) ListZones(ctx context.Context, r ListZonesRequest) (*ListZonesResponse, error) {
	m.record("ListZones", r)
	if m.listErr != nil {
		return nil, m.listErr
	}
	if m.listResp != nil {
		return m.listResp, nil
	}
	return &ListZonesResponse{}, nil
}

func (m *mockClient) CreateRecord(ctx context.Context, zoneID string, r CreateRecordRequest) (*Record, error) {
	m.record("CreateRecord", r)
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.createOK != nil {
		return m.createOK, nil
	}
	return &Record{ID: 1, Name: r.Name, Type: r.Type, Value: r.Value, TTLSeconds: r.TTLSeconds}, nil
}

func (m *mockClient) UpdateRecord(ctx context.Context, zoneID int64, recordID int64, r UpdateRecordRequest) error {
	m.record("UpdateRecord", r)
	return m.updateErr
}

func (m *mockClient) DeleteRecord(ctx context.Context, zoneID int64, recordID int64) error {
	m.record("DeleteRecord", struct {
		ZoneID, RecordID int64
	}{zoneID, recordID})
	return m.deleteErr
}
