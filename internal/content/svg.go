package content

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// MinFontSize es el tamaño de letra más pequeño permitido en las ilustraciones.
// Por debajo de 13 no se lee bien en el móvil (ver CONTENIDO.md > Ilustraciones).
const MinFontSize = 13

// ValidateSVG comprueba que una ilustración cumple las reglas de CONTENIDO.md
// que se pueden comprobar automáticamente. Devuelve la lista de problemas;
// si está vacía, el SVG es correcto.
//
// Recorre el XML con un decodificador por eventos (xml.Decoder.Token): va
// recibiendo "abre etiqueta", "texto", "cierra etiqueta"... Así podemos saber
// en qué grupo <g> estamos y qué tamaño de letra hereda cada <text>.
func ValidateSVG(data []byte) []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	dec := xml.NewDecoder(bytes.NewReader(data))

	// Pila con el font-size de cada elemento abierto. Un hijo sin font-size
	// propio hereda el de su padre, igual que hace el navegador.
	// 0 significa "ninguno definido todavía".
	sizes := []float64{}
	depth := 0 // 1 = la propia <svg>, 2 = sus hijos directos

	var (
		sawRoot, sawTitle, inTitle bool
		titleText, styleText       strings.Builder
		inStyle                    bool
		texts                      int
	)

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			add("XML inválido: %v", err)
			return problems
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			parent := 0.0
			if len(sizes) > 0 {
				parent = sizes[len(sizes)-1]
			}
			size := parent
			if s, ok := fontSize(t); ok {
				size = s
			}
			sizes = append(sizes, size)

			switch t.Name.Local {
			case "svg":
				if depth == 1 {
					sawRoot = true
					checkViewBox(t, add)
				}
			case "title":
				if depth == 2 { // solo cuenta el <title> hijo directo de <svg>
					sawTitle, inTitle = true, true
				}
			case "style":
				inStyle = true
			case "text":
				texts++
				switch {
				case size == 0:
					add("texto %d: sin font-size (ponlo en el <text> o en su <g>)", texts)
				case size < MinFontSize:
					add("texto %d: font-size %g, el mínimo es %d", texts, size, MinFontSize)
				}
			}

		case xml.CharData:
			if inTitle {
				titleText.Write(t)
			}
			if inStyle {
				styleText.Write(t)
			}

		case xml.EndElement:
			depth--
			sizes = sizes[:len(sizes)-1]
			switch t.Name.Local {
			case "title":
				inTitle = false
			case "style":
				inStyle = false
			}
		}
	}

	if !sawRoot {
		add("no es un SVG (falta la etiqueta <svg>)")
		return problems
	}
	if !sawTitle || strings.TrimSpace(titleText.String()) == "" {
		add("falta <title> con texto (lo leen los lectores de pantalla)")
	}
	// Quitamos los espacios para aceptar "prefers-color-scheme: dark" y "prefers-color-scheme:dark".
	css := strings.ReplaceAll(styleText.String(), " ", "")
	if !strings.Contains(css, "prefers-color-scheme:dark") {
		add("falta el modo oscuro (@media (prefers-color-scheme: dark) dentro de <style>)")
	}
	return problems
}

// checkViewBox exige un lienzo de 360 de ancho: "0 0 360 <alto>".
func checkViewBox(t xml.StartElement, add func(string, ...any)) {
	vb := attr(t, "viewBox")
	f := strings.Fields(vb)
	if len(f) != 4 || f[0] != "0" || f[1] != "0" || f[2] != "360" {
		add("viewBox %q: debe ser \"0 0 360 <alto>\"", vb)
		return
	}
	if h, err := strconv.ParseFloat(f[3], 64); err != nil || h <= 0 {
		add("viewBox %q: el alto debe ser un número positivo", vb)
	}
}

// fontSize lee el tamaño de letra de un elemento, sea del atributo
// font-size="15" o de style="font-size:15px".
func fontSize(t xml.StartElement) (float64, bool) {
	v := attr(t, "font-size")
	if v == "" {
		for _, decl := range strings.Split(attr(t, "style"), ";") {
			k, val, ok := strings.Cut(decl, ":")
			if ok && strings.TrimSpace(k) == "font-size" {
				v = val
			}
		}
	}
	v = strings.TrimSuffix(strings.TrimSpace(v), "px")
	if v == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
