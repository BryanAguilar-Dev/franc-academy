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

func TestReviewBoxes(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "p.json"))
	dia := func(d int) { s.now = func() time.Time { return time.Date(2026, 11, d, 12, 0, 0, 0, time.UTC) } }
	responder := func(ok bool) {
		t.Helper()
		if _, err := s.Apply(Event{Type: "review", Ref: "x", Score: 0, Total: 1, Results: []Result{{Key: "p1", Correct: ok}}}); err != nil {
			t.Fatal(err)
		}
	}
	caja := func(wantBox int, wantDue string) {
		t.Helper()
		it := s.Snapshot().Review["p1"]
		if wantBox == 0 {
			if it != nil {
				t.Fatalf("debía estar aprendida, sigue en %+v", it)
			}
			return
		}
		if it == nil || it.Box != wantBox || it.Due != wantDue {
			t.Fatalf("esperaba caja %d para %s, obtuve %+v", wantBox, wantDue, it)
		}
	}

	dia(1)
	responder(false)
	caja(1, "2026-11-02")
	responder(true) // acertar antes de que toque no la sube
	caja(1, "2026-11-02")

	dia(2)
	responder(true)
	caja(2, "2026-11-05")
	dia(5)
	responder(true)
	caja(3, "2026-11-12")
	dia(12)
	responder(false) // un fallo la devuelve a la caja 1
	caja(1, "2026-11-13")

	dia(13)
	responder(true)
	caja(2, "2026-11-16")
	dia(16)
	responder(true)
	caja(3, "2026-11-23")
	dia(23)
	responder(true)
	caja(4, "2026-12-14")
	dia(28) // llegar tarde no pasa nada: sigue pendiente
	if due := s.Snapshot().Due("2026-11-28"); len(due) != 0 {
		t.Fatalf("todavía no toca: %v", due)
	}
	s.now = func() time.Time { return time.Date(2026, 12, 20, 12, 0, 0, 0, time.UTC) }
	if due := s.Snapshot().Due("2026-12-20"); len(due) != 1 {
		t.Fatalf("debía tocar repasarla: %v", due)
	}
	responder(true)
	caja(0, "") // acertada en la última caja: aprendida

	if n := len(s.Snapshot().History); n == 0 || s.Snapshot().History[n-1].Results != nil {
		t.Fatal("el historial no debe guardar cada respuesta")
	}
}
