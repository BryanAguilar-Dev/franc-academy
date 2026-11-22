// Package content carga y valida el catálogo de lecciones.
//
// El contenido vive en archivos JSON (content/<categoría>/<NN-nombre>.json),
// separado de la interfaz. Para añadir una lección basta con crear un archivo
// nuevo: este paquete lo encuentra, lo valida y lo sirve.
package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Category es una de las áreas de conocimiento (Universo, Historia...).
type Category struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Order       int    `json:"order"`
}

// Lesson es una lección completa con su test.
type Lesson struct {
	ID           string        `json:"id"`
	Category     string        `json:"category"`
	Order        int           `json:"order"`
	Title        string        `json:"title"`
	Minutes      int           `json:"minutes"`
	Intro        string        `json:"intro"`
	Illustration *Illustration `json:"illustration,omitempty"`
	Sections     []Section     `json:"sections"`
	Curiosities  []string      `json:"curiosities"`
	KeyConcepts  []Concept     `json:"keyConcepts"`
	Summary      []string      `json:"summary"`
	Sources      []Source      `json:"sources"`
	Questions    []Question    `json:"questions"`
}

// Illustration apunta a un SVG guardado junto a la lección.
type Illustration struct {
	File    string `json:"file"`
	Alt     string `json:"alt"`
	Caption string `json:"caption"`
}

// Section es un apartado de la explicación.
type Section struct {
	Heading string   `json:"heading"`
	Body    []string `json:"body"`
	Example string   `json:"example,omitempty"`
}

// Concept es un término clave con su definición.
type Concept struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// Source es una referencia consultable.
type Source struct {
	Title     string `json:"title"`
	Publisher string `json:"publisher"`
	URL       string `json:"url"`
}

// Question es una pregunta tipo test. Answer es el índice de la opción correcta.
type Question struct {
	Q       string   `json:"q"`
	Options []Option `json:"options"`
	Answer  int      `json:"answer"`
}

// Option es una respuesta posible y la explicación de por qué es (o no) correcta.
type Option struct {
	Text string `json:"text"`
	Why  string `json:"why"`
}

// Catalog es todo el contenido cargado y validado.
type Catalog struct {
	Categories []Category
	Lessons    []Lesson // ordenadas por categoría y número
	byID       map[string]int
}

// Lesson devuelve una lección por su id.
func (c *Catalog) Lesson(id string) (Lesson, bool) {
	i, ok := c.byID[id]
	if !ok {
		return Lesson{}, false
	}
	return c.Lessons[i], true
}

// Load lee categories.json y todas las lecciones de fsys y las valida.
// Si alguna lección es incorrecta devuelve un error que las enumera todas.
func Load(fsys fs.FS) (*Catalog, error) {
	raw, err := fs.ReadFile(fsys, "categories.json")
	if err != nil {
		return nil, fmt.Errorf("leer categories.json: %w", err)
	}
	var cats []Category
	if err := json.Unmarshal(raw, &cats); err != nil {
		return nil, fmt.Errorf("categories.json: %w", err)
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].Order < cats[j].Order })

	known := map[string]bool{}
	for _, c := range cats {
		known[c.ID] = true
	}

	cat := &Catalog{Categories: cats, byID: map[string]int{}}
	var problems []string
	for _, c := range cats {
		files, err := fs.Glob(fsys, path.Join(c.ID, "*.json"))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			raw, err := fs.ReadFile(fsys, f)
			if err != nil {
				return nil, err
			}
			var l Lesson
			dec := json.NewDecoder(strings.NewReader(string(raw)))
			dec.DisallowUnknownFields() // un campo mal escrito es un error, no se ignora
			if err := dec.Decode(&l); err != nil {
				problems = append(problems, fmt.Sprintf("%s: JSON inválido: %v", f, err))
				continue
			}
			for _, p := range Validate(l, known) {
				problems = append(problems, f+": "+p)
			}
			if l.Category != c.ID {
				problems = append(problems, fmt.Sprintf("%s: category %q no coincide con la carpeta %q", f, l.Category, c.ID))
			}
			if l.Illustration != nil {
				if _, err := fs.Stat(fsys, path.Join(c.ID, l.Illustration.File)); err != nil {
					problems = append(problems, fmt.Sprintf("%s: no existe la ilustración %s", f, l.Illustration.File))
				}
			}
			if _, dup := cat.byID[l.ID]; dup {
				problems = append(problems, fmt.Sprintf("%s: id repetido %q", f, l.ID))
				continue
			}
			cat.byID[l.ID] = len(cat.Lessons)
			cat.Lessons = append(cat.Lessons, l)
		}
	}
	if len(problems) > 0 {
		return nil, errors.New("contenido con errores:\n  - " + strings.Join(problems, "\n  - "))
	}

	order := map[string]int{}
	for _, c := range cats {
		order[c.ID] = c.Order
	}
	sort.SliceStable(cat.Lessons, func(i, j int) bool {
		a, b := cat.Lessons[i], cat.Lessons[j]
		if a.Category != b.Category {
			return order[a.Category] < order[b.Category]
		}
		return a.Order < b.Order
	})
	for i, l := range cat.Lessons {
		cat.byID[l.ID] = i
	}
	return cat, nil
}

// Validate comprueba que una lección cumple las reglas de calidad mínimas.
// Devuelve la lista de problemas encontrados (vacía si está bien).
func Validate(l Lesson, categories map[string]bool) []string {
	var p []string
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }

	if l.ID == "" || strings.ContainsAny(l.ID, " /") {
		add("id vacío o con espacios/barras: %q", l.ID)
	}
	if !categories[l.Category] {
		add("categoría desconocida %q", l.Category)
	}
	if strings.TrimSpace(l.Title) == "" || strings.TrimSpace(l.Intro) == "" {
		add("faltan título o introducción")
	}
	if l.Minutes < 5 || l.Minutes > 10 {
		add("duración %d min fuera de 5-10", l.Minutes)
	}
	if len(l.Sections) < 3 {
		add("necesita al menos 3 apartados (tiene %d)", len(l.Sections))
	}
	examples := 0
	for i, s := range l.Sections {
		if strings.TrimSpace(s.Heading) == "" || len(s.Body) == 0 {
			add("apartado %d sin título o sin texto", i+1)
		}
		if s.Example != "" {
			examples++
		}
	}
	if examples < 2 {
		add("necesita al menos 2 ejemplos (tiene %d)", examples)
	}
	if len(l.Curiosities) < 2 {
		add("necesita al menos 2 curiosidades")
	}
	if len(l.KeyConcepts) < 3 {
		add("necesita al menos 3 conceptos clave")
	}
	if len(l.Summary) < 3 {
		add("el resumen necesita al menos 3 puntos")
	}
	if len(l.Sources) < 2 {
		add("necesita al menos 2 fuentes")
	}
	for _, s := range l.Sources {
		if !strings.HasPrefix(s.URL, "https://") || s.Title == "" || s.Publisher == "" {
			add("fuente incompleta o sin https: %q", s.URL)
		}
	}
	if len(l.Questions) != 10 {
		add("necesita exactamente 10 preguntas (tiene %d)", len(l.Questions))
	}
	for i, q := range l.Questions {
		n := i + 1
		if strings.TrimSpace(q.Q) == "" {
			add("pregunta %d vacía", n)
		}
		if len(q.Options) < 3 || len(q.Options) > 4 {
			add("pregunta %d: necesita 3 o 4 opciones", n)
		}
		if q.Answer < 0 || q.Answer >= len(q.Options) {
			add("pregunta %d: answer fuera de rango", n)
		}
		seen := map[string]bool{}
		for j, o := range q.Options {
			if strings.TrimSpace(o.Text) == "" || strings.TrimSpace(o.Why) == "" {
				add("pregunta %d, opción %d: falta texto o explicación", n, j+1)
			}
			key := strings.ToLower(strings.TrimSpace(o.Text))
			if seen[key] {
				add("pregunta %d: opción repetida %q", n, o.Text)
			}
			seen[key] = true
		}
	}
	return p
}
