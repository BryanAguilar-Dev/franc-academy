// Package academy incrusta el contenido y la web dentro del binario.
// Así el programa es un único archivo: no hay que copiar carpetas al servidor.
package academy

import (
	"embed"
	"io/fs"
)

//go:embed all:content
var contentFS embed.FS

//go:embed all:web
var webFS embed.FS

// Content devuelve la carpeta content/.
func Content() fs.FS { sub, _ := fs.Sub(contentFS, "content"); return sub }

// Web devuelve la carpeta web/.
func Web() fs.FS { sub, _ := fs.Sub(webFS, "web"); return sub }
