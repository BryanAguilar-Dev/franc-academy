# Cómo escribir una lección

Cada lección es un archivo `content/<categoría>/NN-nombre.json`. El servidor
lo encuentra solo. Después de añadirlo, ejecuta `go test ./...`: si falta algo,
el test te dice qué.

## Estructura

```json
{
  "id": "universo-04-sol",          // único, sin espacios
  "category": "universo",           // debe coincidir con la carpeta
  "order": 4,                       // posición dentro de la categoría
  "title": "…",
  "minutes": 8,                     // entre 5 y 10
  "intro": "…",
  "illustration": {                 // opcional: SVG en la misma carpeta
    "file": "04-sol.svg", "alt": "descripción para lectores de pantalla", "caption": "…"
  },
  "sections": [                     // mínimo 3, con al menos 2 ejemplos
    { "heading": "…", "body": ["párrafo", "párrafo"], "example": "opcional",
      "code": "opcional: código que se muestra tal cual" }
  ],
  "practice": { "task": "…", "code": "…" },  // opcional: ejercicio para el Go Playground
  "curiosities": ["…", "…"],        // mínimo 2
  "keyConcepts": [{ "term": "…", "definition": "…" }],   // mínimo 3
  "summary": ["…", "…", "…"],       // mínimo 3
  "sources": [{ "title": "…", "publisher": "…", "url": "https://…" }],  // mínimo 2
  "questions": [                    // exactamente 10
    {
      "q": "…",
      "answer": 2,                  // índice de la opción correcta (empieza en 0)
      "options": [                  // 3 o 4, cada una con su explicación
        { "text": "…", "why": "por qué no es correcta" },
        { "text": "…", "why": "…" },
        { "text": "…", "why": "Correcto. …" },
        { "text": "…", "why": "…" }
      ]
    }
  ]
}
```

(Los comentarios `//` son solo para esta guía: en el archivo real no se permiten.)

## Reglas de calidad

1. **Original.** Se investiga en las fuentes y se redacta con palabras propias.
   Nunca se copian párrafos.
2. **Nada inventado.** Cada dato, fecha o cifra importante debe estar en alguna
   de las fuentes citadas o ser conocimiento consolidado. Ante la duda, se quita.
3. **Fuentes fiables y enlazables:** NASA, ESA, OpenStax, Khan Academy, MIT OCW,
   universidades, museos, divulgación de calidad.
4. **Para aprender, no para memorizar datos.** Cada apartado explica un porqué.
   Los ejemplos conectan con la vida cotidiana.
5. **Preguntas que comprueban comprensión.** Evita "¿en qué año…?" salvo que la
   fecha importe. Prefiere "¿por qué…?", "¿qué pasaría si…?", "¿qué significa…?".
   Una sola respuesta claramente correcta; las incorrectas, plausibles; cada
   opción explica por qué es o no correcta.
6. **Varía la posición de la respuesta correcta.** El test falla si más de 5 de
   10 están en la misma posición (además, la app mezcla las opciones).
7. **Español de España**, frases cortas, para un adulto sin estudios previos del tema.
8. **Números en español:** coma decimal, punto de miles, y cuidado con
   "billion" (inglés, 10⁹) frente a "billón" (español, 10¹²).

## Ilustraciones

SVG simples hechos a mano, con `<title>`, un `alt` descriptivo y colores que
funcionen en modo claro y oscuro (con `@media (prefers-color-scheme: dark)`
dentro del propio SVG). Nada de imágenes copiadas sin licencia.

Diseño pensado primero para el móvil:
- Lienzo de 360 de ancho (`viewBox="0 0 360 …"`); el alto, el que haga falta.
- Texto principal de 15-16 y notas de 13 como mínimo.
- Disposición vertical: una cosa debajo de otra, sin filas largas.
- Ningún texto a menos de 20 del borde. Comprueba que nada se sale.

`go test ./...` comprueba automáticamente: `viewBox="0 0 360 <alto>"`,
`<title>` con texto, `alt` en el JSON, modo oscuro en el `<style>` y que
ningún `<text>` tenga menos de 13. El tamaño se pone con `font-size` en el
`<text>` o en su `<g>` (no en clases CSS: el test no las lee). Lo del borde
y la disposición vertical no se comprueban solos: revísalo con `rsvg-convert`.

## Lecciones de Go

- Cada apartado con `code` cuenta como ejemplo. En `code` los saltos de línea
  son `\n` y las tabulaciones `\t` (como escribe `gofmt`).
- Todo el código debe compilar y dar la salida que dice el texto: se prueba en
  el Go Playground antes de publicarlo.
- Dentro de `body`, el texto entre comillas invertidas (`` `así` ``) se
  muestra como código.
- Cada lección termina con un `practice`: un enunciado corto y un programa
  completo de partida (con `package main` y `func main`).
- Fuentes oficiales: go.dev (Tour of Go, Effective Go, la especificación).
