package progress

import (
	"path/filepath"
	"testing"
	"time"
)

func TestQuizCompletesLessonAndPersists(t *testing.T) {
	file := filepath.Join(t.TempDir(), "progress.json")
	s, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC) }

	if _, err := s.Apply(Event{Type: "quiz", Ref: "l1", Score: 6, Total: 10}); err != nil {
		t.Fatal(err)
	}
	if d := s.Snapshot(); d.Lessons["l1"].CompletedAt != nil {
		t.Fatal("6/10 no debe completar la lección (mínimo 7)")
	}
	s.Apply(Event{Type: "quiz", Ref: "l1", Score: 8, Total: 10})
	s.Apply(Event{Type: "quiz", Ref: "l1", Score: 5, Total: 10})

	// Reabrimos el archivo, como si el contenedor se hubiera reiniciado.
	s2, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	st := s2.Snapshot().Lessons["l1"]
	if st.CompletedAt == nil || st.BestScore != 8 || st.Attempts != 3 {
		t.Fatalf("estado inesperado tras reabrir: %+v", st)
	}
	if n := len(s2.Snapshot().History); n != 3 {
		t.Fatalf("historial: esperaba 3 eventos, hay %d", n)
	}
}

func TestDailyCountsOncePerDay(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "p.json"))
	s.Apply(Event{Type: "daily", Ref: "2026-11-22", Score: 2, Total: 5})
	s.Apply(Event{Type: "daily", Ref: "2026-11-22", Score: 5, Total: 5})
	if got := s.Snapshot().Daily["2026-11-22"].Score; got != 2 {
		t.Fatalf("el reto diario cuenta el primer intento: esperaba 2, obtuve %d", got)
	}
}

func TestRejectsBadEvents(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "p.json"))
	bad := []Event{
		{Type: "hack", Ref: "x", Score: 1, Total: 1},
		{Type: "quiz", Ref: "", Score: 1, Total: 1},
		{Type: "quiz", Ref: "x", Score: 11, Total: 10},
		{Type: "exam", Ref: "x", Score: 1, Total: 0},
	}
	for _, e := range bad {
		if _, err := s.Apply(e); err == nil {
			t.Errorf("debería rechazar %+v", e)
		}
	}
}
