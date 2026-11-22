// Package server expone la web y la API de Franc Academy.
package server

import (
	"encoding/json"
	"hash/fnv"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/BryanAguilar-Dev/franc-academy/internal/content"
	"github.com/BryanAguilar-Dev/franc-academy/internal/progress"
)

const (
	dailySize = 5
	examSize  = 20
)

// Server agrupa lo que necesitan los manejadores HTTP.
type Server struct {
	Catalog  *content.Catalog
	Content  fs.FS // carpeta content (para las ilustraciones)
	Web      fs.FS // carpeta web (HTML, CSS, JS)
	Progress *progress.Store
	Location *time.Location // zona horaria para el "día" del reto diario
	Now      func() time.Time
}

// Handler devuelve el enrutador con todas las rutas.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/catalog", s.catalog)
	mux.HandleFunc("GET /api/lessons/{id}", s.lesson)
	mux.HandleFunc("GET /api/progress", s.getProgress)
	mux.HandleFunc("POST /api/events", s.postEvent)
	mux.HandleFunc("GET /api/daily", s.daily)
	mux.HandleFunc("GET /api/exam/{category}", s.exam)
	mux.HandleFunc("GET /media/{category}/{file}", s.media)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.Handle("GET /", http.FileServerFS(s.Web))
	return securityHeaders(mux)
}

// LessonSummary es lo que necesita el panel y el buscador (sin el texto completo).
type LessonSummary struct {
	ID       string   `json:"id"`
	Category string   `json:"category"`
	Order    int      `json:"order"`
	Title    string   `json:"title"`
	Minutes  int      `json:"minutes"`
	Intro    string   `json:"intro"`
	Concepts []string `json:"concepts"`
}

func (s *Server) catalog(w http.ResponseWriter, _ *http.Request) {
	out := struct {
		Categories []content.Category `json:"categories"`
		Lessons    []LessonSummary    `json:"lessons"`
	}{Categories: s.Catalog.Categories, Lessons: []LessonSummary{}}
	for _, l := range s.Catalog.Lessons {
		sum := LessonSummary{ID: l.ID, Category: l.Category, Order: l.Order, Title: l.Title, Minutes: l.Minutes, Intro: l.Intro}
		for _, c := range l.KeyConcepts {
			sum.Concepts = append(sum.Concepts, c.Term)
		}
		out.Lessons = append(out.Lessons, sum)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) lesson(w http.ResponseWriter, r *http.Request) {
	l, ok := s.Catalog.Lesson(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "lección no encontrada"})
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) getProgress(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Progress.Snapshot())
}

func (s *Server) postEvent(w http.ResponseWriter, r *http.Request) {
	var e progress.Event
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&e); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}
	// Solo se aceptan referencias que existen: así el archivo no se llena de basura.
	if !s.validRef(e) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "referencia desconocida"})
		return
	}
	d, err := s.Progress.Apply(e)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) validRef(e progress.Event) bool {
	switch e.Type {
	case "read", "quiz":
		_, ok := s.Catalog.Lesson(e.Ref)
		return ok
	case "exam":
		for _, c := range s.Catalog.Categories {
			if c.ID == e.Ref {
				return true
			}
		}
		return false
	case "daily":
		return e.Ref == s.today()
	}
	return false
}

// QuizItem es una pregunta junto a la lección de la que sale.
type QuizItem struct {
	LessonID    string `json:"lessonId"`
	LessonTitle string `json:"lessonTitle"`
	content.Question
}

func (s *Server) today() string {
	return s.Now().In(s.Location).Format("2006-01-02")
}

// daily elige 5 preguntas para hoy. La elección depende solo de la fecha, así
// que el reto es el mismo en el móvil y en el PC durante todo el día.
// Prioriza lecciones que ya has leído: el reto sirve para repasar.
func (s *Server) daily(w http.ResponseWriter, _ *http.Request) {
	date := s.today()
	prog := s.Progress.Snapshot()
	var studied, all []QuizItem
	for _, l := range s.Catalog.Lessons {
		_, seen := prog.Lessons[l.ID]
		for _, q := range l.Questions {
			it := QuizItem{LessonID: l.ID, LessonTitle: l.Title, Question: q}
			all = append(all, it)
			if seen {
				studied = append(studied, it)
			}
		}
	}
	pool := all
	if len(studied) >= dailySize {
		pool = studied
	}
	h := fnv.New64a()
	h.Write([]byte(date))
	items := pick(pool, dailySize, rand.New(rand.NewSource(int64(h.Sum64()))))
	done := prog.Daily[date]
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "questions": items, "done": done})
}

func (s *Server) exam(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("category")
	var pool []QuizItem
	for _, l := range s.Catalog.Lessons {
		if l.Category != cat {
			continue
		}
		for _, q := range l.Questions {
			pool = append(pool, QuizItem{LessonID: l.ID, LessonTitle: l.Title, Question: q})
		}
	}
	if len(pool) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "esta categoría aún no tiene lecciones"})
		return
	}
	items := pick(pool, examSize, rand.New(rand.NewSource(s.Now().UnixNano())))
	writeJSON(w, http.StatusOK, map[string]any{"category": cat, "questions": items})
}

func pick(pool []QuizItem, n int, rng *rand.Rand) []QuizItem {
	cp := append([]QuizItem(nil), pool...)
	rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}

// media sirve las ilustraciones SVG de content/<categoría>/.
func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	cat, file := r.PathValue("category"), r.PathValue("file")
	if strings.Contains(cat, "..") || !strings.HasSuffix(file, ".svg") || strings.Contains(file, "..") {
		http.NotFound(w, r)
		return
	}
	raw, err := fs.ReadFile(s.Content, path.Join(cat, file))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(raw)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("escribir respuesta: %v", err)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
