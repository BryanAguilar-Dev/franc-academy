package content_test

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	academy "github.com/BryanAguilar-Dev/franc-academy"
	"github.com/BryanAguilar-Dev/franc-academy/internal/content"
)

// TestRealContent valida TODAS las lecciones del repositorio.
// Si añades una lección incompleta, este test falla y te dice qué falta.
func TestRealContent(t *testing.T) {
	cat, err := content.Load(academy.Content())
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Categories) != 6 {
		t.Errorf("esperaba 6 categorías, hay %d", len(cat.Categories))
	}
	if len(cat.Lessons) == 0 {
		t.Error("no hay ninguna lección")
	}
	// Las respuestas correctas no deben estar siempre en la misma posición.
	for _, l := range cat.Lessons {
		pos := map[int]int{}
		for _, q := range l.Questions {
			pos[q.Answer]++
		}
		for p, n := range pos {
			if n > 5 {
				t.Errorf("%s: %d de 10 respuestas correctas están en la posición %d", l.ID, n, p)
			}
		}
	}
}

func goodLesson() content.Lesson {
	q := content.Question{Q: "¿?", Answer: 1, Options: []content.Option{
		{Text: "a", Why: "no"}, {Text: "b", Why: "sí"}, {Text: "c", Why: "no"},
	}}
	l := content.Lesson{
		ID: "x-01", Category: "x", Order: 1, Title: "T", Minutes: 7, Intro: "I",
		Sections: []content.Section{
			{Heading: "1", Body: []string{"a"}, Example: "e"},
			{Heading: "2", Body: []string{"a"}, Example: "e"},
			{Heading: "3", Body: []string{"a"}},
		},
		Curiosities: []string{"c1", "c2"},
		KeyConcepts: []content.Concept{{"a", "a"}, {"b", "b"}, {"c", "c"}},
		Summary:     []string{"1", "2", "3"},
		Sources: []content.Source{
			{Title: "s", Publisher: "p", URL: "https://a"},
			{Title: "s", Publisher: "p", URL: "https://b"},
		},
	}
	for i := 0; i < 10; i++ {
		l.Questions = append(l.Questions, q)
	}
	return l
}

func TestValidate(t *testing.T) {
	cats := map[string]bool{"x": true}
	if p := content.Validate(goodLesson(), cats); len(p) != 0 {
		t.Fatalf("una lección correcta no debería tener problemas: %v", p)
	}

	tests := []struct {
		name   string
		break_ func(*content.Lesson)
		want   string
	}{
		{"pocas preguntas", func(l *content.Lesson) { l.Questions = l.Questions[:9] }, "exactamente 10"},
		{"answer fuera", func(l *content.Lesson) { l.Questions[0].Answer = 7 }, "fuera de rango"},
		{"sin explicación", func(l *content.Lesson) { l.Questions[2].Options[0].Why = "" }, "explicación"},
		{"opción repetida", func(l *content.Lesson) { l.Questions[3].Options[2].Text = "A" }, "repetida"},
		{"fuente http", func(l *content.Lesson) { l.Sources[0].URL = "http://a" }, "https"},
		{"una fuente", func(l *content.Lesson) { l.Sources = l.Sources[:1] }, "2 fuentes"},
		{"duración", func(l *content.Lesson) { l.Minutes = 30 }, "5-10"},
		{"sin ejemplos", func(l *content.Lesson) { l.Sections[0].Example = "" }, "ejemplos"},
		{"categoría", func(l *content.Lesson) { l.Category = "y" }, "categoría"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := goodLesson()
			// copia profunda de las preguntas para no romper las demás
			raw, _ := json.Marshal(l)
			_ = json.Unmarshal(raw, &l)
			tt.break_(&l)
			p := strings.Join(content.Validate(l, cats), " | ")
			if !strings.Contains(p, tt.want) {
				t.Errorf("esperaba un problema con %q, obtuve: %q", tt.want, p)
			}
		})
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	raw, _ := json.Marshal(goodLesson())
	bad := strings.Replace(string(raw), `"title"`, `"titel":"x","title"`, 1)
	fsys := fstest.MapFS{
		"categories.json": {Data: []byte(`[{"id":"x","order":1,"title":"X"}]`)},
		"x/01.json":       {Data: []byte(bad)},
	}
	if _, err := content.Load(fsys); err == nil || !strings.Contains(err.Error(), "titel") {
		t.Fatalf("un campo mal escrito debe dar error, obtuve %v", err)
	}
}

// goodSVG cumple todas las reglas. Cada caso de TestValidateSVG rompe una.
const goodSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 360 200">
  <title>Ejemplo</title>
  <style>.a{fill:#000} @media (prefers-color-scheme: dark){.a{fill:#fff}}</style>
  <g font-size="15"><text>hereda 15</text><text font-size="13">nota</text></g>
  <text style="font-size:16px">grande</text>
</svg>`

func TestValidateSVG(t *testing.T) {
	if p := content.ValidateSVG([]byte(goodSVG)); len(p) != 0 {
		t.Fatalf("un SVG correcto no debería tener problemas: %v", p)
	}

	tests := []struct {
		name, old, new, want string
	}{
		{"ancho", `0 0 360 200`, `0 0 400 200`, "viewBox"},
		{"sin viewBox", ` viewBox="0 0 360 200"`, ``, "viewBox"},
		{"sin title", `<title>Ejemplo</title>`, ``, "<title>"},
		{"title vacío", `<title>Ejemplo</title>`, `<title> </title>`, "<title>"},
		{"letra pequeña", `font-size="13"`, `font-size="12"`, "mínimo es 13"},
		{"hereda pequeña", `<g font-size="15">`, `<g font-size="11">`, "mínimo es 13"},
		{"style pequeña", `font-size:16px`, `font-size:10px`, "mínimo es 13"},
		{"sin tamaño", `<g font-size="15">`, `<g>`, "sin font-size"},
		{"sin modo oscuro", `@media (prefers-color-scheme: dark){.a{fill:#fff}}`, ``, "modo oscuro"},
		{"XML roto", `</svg>`, ``, "XML inválido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bad := strings.Replace(goodSVG, tt.old, tt.new, 1)
			p := strings.Join(content.ValidateSVG([]byte(bad)), " | ")
			if !strings.Contains(p, tt.want) {
				t.Errorf("esperaba un problema con %q, obtuve: %q", tt.want, p)
			}
		})
	}
}
