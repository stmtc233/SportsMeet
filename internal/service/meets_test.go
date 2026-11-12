package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/store"
)

func newTestService(t *testing.T) (*Service, *[]Change) {
	t.Helper()
	st, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	var changes []Change
	return New(st, func(c Change) { changes = append(changes, c) }), &changes
}

func TestCreateMeet(t *testing.T) {
	ctx := context.Background()
	svc, changes := newTestService(t)

	m, err := svc.CreateMeet(ctx, store.SourceUI, MeetInput{Name: " 秋季运动会 ", StartDate: "2026-11-12"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "秋季运动会" || len(*changes) != 1 || (*changes)[0].MeetID != m.ID {
		t.Fatalf("meet = %+v, changes = %+v", m, *changes)
	}
	list, err := svc.ListMeets(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListMeets = %+v, %v", list, err)
	}

	_, err = svc.CreateMeet(ctx, store.SourceMCP, MeetInput{Name: "x", StartDate: "2026-11-12", EndDate: "2026-11-11"})
	var ie *InputError
	if !errors.As(err, &ie) {
		t.Fatalf("end before start: err = %v, want InputError", err)
	}
	if len(*changes) != 1 {
		t.Fatal("a rejected write notified listeners")
	}
}
