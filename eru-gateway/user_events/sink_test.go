package user_events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-store/store"
	"github.com/jmoiron/sqlx"
)

type fakeStore struct {
	store.Store
	conn        *sqlx.DB
	createCalls int
	createErr   error
	connectOn   int
}

func (f *fakeStore) GetConn() *sqlx.DB { return f.conn }

func (f *fakeStore) CreateConn() error {
	f.createCalls++
	if f.createErr != nil {
		return f.createErr
	}
	if f.connectOn > 0 && f.createCalls >= f.connectOn {
		f.conn = &sqlx.DB{}
	}
	return nil
}

func TestNewDbSinkOpensConnectionLazily(t *testing.T) {
	s := &fakeStore{connectOn: 1}

	sink, err := newDbSink(context.Background(), s, "t", false)
	if err != nil {
		t.Fatalf("expected the sink to open the connection itself, got %v", err)
	}
	if sink == nil {
		t.Fatal("expected a sink")
	}
	if s.createCalls != 1 {
		t.Errorf("expected CreateConn to be called once, got %d", s.createCalls)
	}
}

func TestNewDbSinkFailsWhenStoreCannotConnect(t *testing.T) {
	s := &fakeStore{createErr: errors.New("dial tcp: connection refused")}

	if _, err := newDbSink(context.Background(), s, "t", false); err == nil {
		t.Error("expected an error when the store cannot connect")
	}
}

func TestNewDbSinkFailsOnStandaloneStore(t *testing.T) {
	s := &fakeStore{}

	_, err := newDbSink(context.Background(), s, "t", false)
	if err == nil {
		t.Fatal("expected an error when the store never yields a connection")
	}
	if !contains(err.Error(), "STORE_TYPE=POSTGRES") {
		t.Errorf("expected the error to name the required store type, got %q", err)
	}
}

func TestNewDbSinkFailsOnNilStore(t *testing.T) {
	if _, err := newDbSink(context.Background(), nil, "t", false); err == nil {
		t.Error("expected an error for a nil store")
	}
}

func TestConnReusesExistingConnection(t *testing.T) {
	s := &fakeStore{conn: &sqlx.DB{}}
	d := &dbSink{store: s, tableName: "t"}

	if _, err := d.conn(); err != nil {
		t.Fatal(err)
	}
	if s.createCalls != 0 {
		t.Errorf("expected no CreateConn call when a connection exists, got %d", s.createCalls)
	}
}

func TestConnRetriesAfterConnectionLost(t *testing.T) {
	s := &fakeStore{conn: &sqlx.DB{}}
	d := &dbSink{store: s, tableName: "t"}

	if _, err := d.conn(); err != nil {
		t.Fatal(err)
	}
	s.conn = nil
	s.connectOn = 1

	if _, err := d.conn(); err != nil {
		t.Fatalf("expected the sink to reconnect, got %v", err)
	}
	if s.createCalls != 1 {
		t.Errorf("expected one reconnect attempt, got %d", s.createCalls)
	}
}

func TestWriteWithNoEventsSkipsConnection(t *testing.T) {
	s := &fakeStore{}
	d := &dbSink{store: s, tableName: "t"}

	if err := d.Write(context.Background(), nil); err != nil {
		t.Errorf("an empty batch must be a no-op, got %v", err)
	}
	if s.createCalls != 0 {
		t.Errorf("an empty batch must not touch the database, got %d calls", s.createCalls)
	}
}

func TestBuildInsertUsesConfiguredTableName(t *testing.T) {
	query, _ := buildInsert("custom_events", []UserEvent{{Status: 200, RequestTime: time.Now()}})
	if !contains(query, "insert into custom_events (") {
		t.Errorf("expected the configured table name in the query, got %q", query)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
