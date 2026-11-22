package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // la imagen de compilación no trae zonas horarias

	academy "github.com/BryanAguilar-Dev/franc-academy"
	"github.com/BryanAguilar-Dev/franc-academy/internal/content"
	"github.com/BryanAguilar-Dev/franc-academy/internal/progress"
)

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	cat, err := content.Load(academy.Content())
	if err != nil {
		t.Fatal(err)
	}
	store, err := progress.Open(filepath.Join(t.TempDir(), "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Europe/Madrid")
	s := &Server{Catalog: cat, Content: academy.Content(), Web: academy.Web(), Progress: store, Location: loc,
		Now: func() time.Time { return time.Date(2026, 11, 22, 23, 30, 0, 0, time.UTC) }} // ya es día 23 en Madrid
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

func get(t *testing.T, url string, v any) int {
	t.Helper()
	r, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if v != nil {
		json.NewDecoder(r.Body).Decode(v)
	}
	return r.StatusCode
}

func post(t *testing.T, url, body string) int {
	t.Helper()
	r, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	return r.StatusCode
}

func TestPagesAndCatalog(t *testing.T) {
	ts, s := newTestServer(t)
	if code := get(t, ts.URL+"/", nil); code != 200 {
		t.Fatalf("la página principal devuelve %d", code)
	}
	var cat struct {
		Lessons []LessonSummary `json:"lessons"`
	}
	get(t, ts.URL+"/api/catalog", &cat)
	if len(cat.Lessons) != len(s.Catalog.Lessons) {
		t.Fatalf("el catálogo tiene %d lecciones, esperaba %d", len(cat.Lessons), len(s.Catalog.Lessons))
	}
	first := s.Catalog.Lessons[0]
	if code := get(t, ts.URL+"/api/lessons/"+first.ID, nil); code != 200 {
		t.Fatalf("lección: %d", code)
	}
	if code := get(t, ts.URL+"/api/lessons/no-existe", nil); code != 404 {
		t.Fatalf("lección inexistente: esperaba 404, obtuve %d", code)
	}
	if first.Illustration != nil {
		if code := get(t, ts.URL+"/media/"+first.Category+"/"+first.Illustration.File, nil); code != 200 {
			t.Fatalf("ilustración: %d", code)
		}
	}
	if code := get(t, ts.URL+"/media/universo/..%2F..%2Fgo.mod", nil); code == 200 {
		t.Fatal("no se deben poder leer archivos fuera de content/")
	}
}

func TestEventsUpdateProgress(t *testing.T) {
	ts, s := newTestServer(t)
	id := s.Catalog.Lessons[0].ID
	if code := post(t, ts.URL+"/api/events", `{"type":"quiz","ref":"`+id+`","score":8,"total":10}`); code != 200 {
		t.Fatalf("evento válido: %d", code)
	}
	if code := post(t, ts.URL+"/api/events", `{"type":"quiz","ref":"inventada","score":8,"total":10}`); code != 400 {
		t.Fatalf("lección inventada: esperaba 400, obtuve %d", code)
	}
	var p progress.Data
	get(t, ts.URL+"/api/progress", &p)
	if p.Lessons[id] == nil || p.Lessons[id].CompletedAt == nil {
		t.Fatal("8/10 debería completar la lección")
	}
}

func TestDailyIsStableAndUsesMadridDate(t *testing.T) {
	ts, _ := newTestServer(t)
	var a, b struct {
		Date      string     `json:"date"`
		Questions []QuizItem `json:"questions"`
	}
	get(t, ts.URL+"/api/daily", &a)
	get(t, ts.URL+"/api/daily", &b)
	if a.Date != "2026-11-23" {
		t.Fatalf("a las 00:30 en Madrid ya es día 23, obtuve %s", a.Date)
	}
	if len(a.Questions) != dailySize {
		t.Fatalf("esperaba %d preguntas, hay %d", dailySize, len(a.Questions))
	}
	for i := range a.Questions {
		if a.Questions[i].Q != b.Questions[i].Q {
			t.Fatal("el reto del día debe ser el mismo en cada petición")
		}
	}
	if code := post(t, ts.URL+"/api/events", `{"type":"daily","ref":"2020-01-01","score":1,"total":5}`); code != 400 {
		t.Fatal("no se debe poder registrar el reto de otro día")
	}
}

func TestExam(t *testing.T) {
	ts, _ := newTestServer(t)
	var e struct {
		Questions []QuizItem `json:"questions"`
	}
	if code := get(t, ts.URL+"/api/exam/universo", &e); code != 200 || len(e.Questions) == 0 {
		t.Fatalf("examen: código %d, %d preguntas", code, len(e.Questions))
	}
	if len(e.Questions) > examSize {
		t.Fatalf("el examen no debe pasar de %d preguntas", examSize)
	}
	if code := get(t, ts.URL+"/api/exam/historia", nil); code != 404 {
		t.Fatal("una categoría sin lecciones debe dar 404")
	}
}

func TestReviewFlow(t *testing.T) {
	ts, s := newTestServer(t)
	l := s.Catalog.Lessons[0]
	fallada, acertada := l.Questions[0].Key, l.Questions[1].Key
	body := `{"type":"quiz","ref":"` + l.ID + `","score":9,"total":10,"results":[` +
		`{"key":"` + fallada + `","correct":false},{"key":"` + acertada + `","correct":true}]}`
	if code := post(t, ts.URL+"/api/events", body); code != 200 {
		t.Fatalf("evento con respuestas: %d", code)
	}

	type repaso struct {
		Date      string     `json:"date"`
		Questions []QuizItem `json:"questions"`
		Pending   int        `json:"pending"`
	}
	var hoy repaso
	get(t, ts.URL+"/api/review", &hoy)
	if hoy.Pending != 0 {
		t.Fatal("una pregunta fallada hoy se repasa mañana, no hoy")
	}

	// Mañana (día 24 en Madrid) ya toca.
	s.Now = func() time.Time { return time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC) }
	var manana repaso
	get(t, ts.URL+"/api/review", &manana)
	if manana.Pending != 1 || len(manana.Questions) != 1 || manana.Questions[0].Key != fallada {
		t.Fatalf("esperaba repasar solo la fallada, obtuve %+v", manana)
	}

	if code := post(t, ts.URL+"/api/events", `{"type":"review","ref":"2026-11-24","score":1,"total":1,"results":[{"key":"`+fallada+`","correct":true}]}`); code != 200 {
		t.Fatalf("evento de repaso: %d", code)
	}
	var p progress.Data
	get(t, ts.URL+"/api/progress", &p)
	if it := p.Review[fallada]; it == nil || it.Box != 2 || it.Due != "2026-11-27" {
		t.Fatalf("tras acertarla debía pasar a la caja 2 para dentro de 3 días: %+v", it)
	}
}

func TestReviewRejectsUnknownQuestions(t *testing.T) {
	ts, s := newTestServer(t)
	id := s.Catalog.Lessons[0].ID
	body := `{"type":"quiz","ref":"` + id + `","score":8,"total":10,"results":[{"key":"` + id + `#inventada","correct":false}]}`
	if code := post(t, ts.URL+"/api/events", body); code != 400 {
		t.Fatalf("una pregunta que no existe debe dar 400, obtuve %d", code)
	}
}
