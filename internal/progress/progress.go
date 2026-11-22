// Package progress guarda el progreso de estudio en un archivo JSON.
//
// El servidor es quien decide cómo cambia el progreso: el navegador solo
// envía eventos ("he hecho el test de tal lección con tal nota"). Así, si
// estudias en el móvil y en el PC a la vez, no se pisan los datos.
package progress

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PassRatio es la nota mínima (en tanto por uno) para dar una lección por completada.
const PassRatio = 0.7

// maxHistory limita el historial para que el archivo no crezca sin fin.
const maxHistory = 2000

// Data es todo el progreso guardado.
type Data struct {
	Lessons    map[string]*LessonState `json:"lessons"`
	Exams      map[string]*Score       `json:"exams"`
	Daily      map[string]*Score       `json:"daily"` // clave: fecha AAAA-MM-DD
	History    []Event                 `json:"history"`
	LastLesson string                  `json:"lastLesson,omitempty"`
}

// LessonState es lo que sabemos de una lección.
type LessonState struct {
	ReadAt      *time.Time `json:"readAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	BestScore   int        `json:"bestScore"`
	Total       int        `json:"total"`
	Attempts    int        `json:"attempts"`
}

// Score es una nota con su fecha.
type Score struct {
	Score int       `json:"score"`
	Total int       `json:"total"`
	At    time.Time `json:"at"`
}

// Event es una acción del usuario. Type: "read", "quiz", "exam" o "daily".
type Event struct {
	Type  string    `json:"type"`
	Ref   string    `json:"ref"` // id de lección, id de categoría o fecha
	Score int       `json:"score"`
	Total int       `json:"total"`
	At    time.Time `json:"at"`
}

// Store guarda Data en un archivo. Es seguro usarlo desde varias peticiones a la vez.
type Store struct {
	mu   sync.Mutex
	path string
	data Data
	now  func() time.Time
}

// Open carga el archivo si existe; si no, empieza con progreso vacío.
func Open(path string) (*Store, error) {
	s := &Store{path: path, now: time.Now}
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, fmt.Errorf("%s está dañado: %w", path, err)
		}
	}
	s.data.init()
	return s, nil
}

func (d *Data) init() {
	if d.Lessons == nil {
		d.Lessons = map[string]*LessonState{}
	}
	if d.Exams == nil {
		d.Exams = map[string]*Score{}
	}
	if d.Daily == nil {
		d.Daily = map[string]*Score{}
	}
}

// Snapshot devuelve una copia del progreso actual.
func (s *Store) Snapshot() Data {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(s.data)
	var cp Data
	_ = json.Unmarshal(raw, &cp)
	cp.init()
	return cp
}

// Apply registra un evento, actualiza el progreso y lo guarda en disco.
func (s *Store) Apply(e Event) (Data, error) {
	if err := e.check(); err != nil {
		return Data{}, err
	}
	s.mu.Lock()
	e.At = s.now().UTC()
	d := &s.data
	switch e.Type {
	case "read":
		st := d.lesson(e.Ref)
		if st.ReadAt == nil {
			t := e.At
			st.ReadAt = &t
		}
		d.LastLesson = e.Ref
	case "quiz":
		st := d.lesson(e.Ref)
		st.Attempts++
		if e.Score >= st.BestScore {
			st.BestScore, st.Total = e.Score, e.Total
		}
		if st.CompletedAt == nil && float64(e.Score) >= PassRatio*float64(e.Total) {
			t := e.At
			st.CompletedAt = &t
		}
		d.LastLesson = e.Ref
	case "exam":
		best(d.Exams, e)
	case "daily":
		// el reto diario cuenta solo la primera vez de cada día
		if _, done := d.Daily[e.Ref]; !done {
			d.Daily[e.Ref] = &Score{Score: e.Score, Total: e.Total, At: e.At}
		}
	}
	d.History = append(d.History, e)
	if len(d.History) > maxHistory {
		d.History = d.History[len(d.History)-maxHistory:]
	}
	err := s.save()
	s.mu.Unlock()
	if err != nil {
		return Data{}, err
	}
	return s.Snapshot(), nil
}

func (e Event) check() error {
	switch e.Type {
	case "read", "quiz", "exam", "daily":
	default:
		return fmt.Errorf("tipo de evento desconocido %q", e.Type)
	}
	if e.Ref == "" || len(e.Ref) > 100 {
		return errors.New("ref vacío o demasiado largo")
	}
	if e.Type != "read" && (e.Total <= 0 || e.Score < 0 || e.Score > e.Total) {
		return errors.New("nota fuera de rango")
	}
	return nil
}

func (d *Data) lesson(id string) *LessonState {
	st, ok := d.Lessons[id]
	if !ok {
		st = &LessonState{}
		d.Lessons[id] = st
	}
	return st
}

func best(m map[string]*Score, e Event) {
	old, ok := m[e.Ref]
	newRatio := float64(e.Score) / float64(e.Total)
	if !ok || newRatio >= float64(old.Score)/float64(old.Total) {
		m[e.Ref] = &Score{Score: e.Score, Total: e.Total, At: e.At}
	}
}

// save escribe en un archivo temporal y luego lo renombra. Si Franc se apaga a
// mitad de escritura, el archivo bueno anterior sigue intacto.
func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
