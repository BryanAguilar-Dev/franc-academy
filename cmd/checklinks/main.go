// checklinks comprueba que las URLs de "sources" de todas las lecciones
// siguen funcionando. Lo ejecuta cada semana .github/workflows/enlaces.yml.
//
// Uso, desde la raíz del repo:
//
//	go run ./cmd/checklinks
//
// Sale con código 1 si algún enlace está roto, para que GitHub marque el
// workflow en rojo y avise por correo.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Solo declaramos lo que necesitamos del JSON; Go ignora el resto.
type leccion struct {
	Sources []struct {
		URL string `json:"url"`
	} `json:"sources"`
}

// resultado de comprobar una URL.
type resultado struct {
	url    string
	estado int   // código HTTP (0 si ni siquiera hubo respuesta)
	err    error // error de red, si lo hubo
	usos   []string
}

func main() {
	usos, err := recogerURLs("content/*/*.json")
	if err != nil {
		log.Fatal(err)
	}
	if len(usos) == 0 {
		log.Fatal("no se encontró ninguna URL: ¿estás en la raíz del repo?")
	}

	resultados := comprobarTodas(usos)

	rotos, avisos := 0, 0
	for _, r := range resultados {
		switch clasificar(r) {
		case "ok":
			continue
		case "aviso":
			avisos++
			fmt.Printf("⚠️  %s\n    %s\n    en: %v\n", describir(r), r.url, r.usos)
		case "roto":
			rotos++
			fmt.Printf("❌ %s\n    %s\n    en: %v\n", describir(r), r.url, r.usos)
		}
	}

	fmt.Printf("\n%d URLs comprobadas: %d rotas, %d avisos.\n", len(resultados), rotos, avisos)
	if rotos > 0 {
		os.Exit(1)
	}
}

// recogerURLs lee las lecciones y devuelve cada URL con los archivos que la
// usan. Usar un mapa quita los duplicados: una fuente citada en tres
// lecciones solo se pide una vez.
func recogerURLs(patron string) (map[string][]string, error) {
	archivos, err := filepath.Glob(patron)
	if err != nil {
		return nil, err
	}
	usos := map[string][]string{}
	for _, ruta := range archivos {
		datos, err := os.ReadFile(ruta)
		if err != nil {
			return nil, err
		}
		var l leccion
		if err := json.Unmarshal(datos, &l); err != nil {
			return nil, fmt.Errorf("%s: %w", ruta, err)
		}
		for _, s := range l.Sources {
			usos[s.URL] = append(usos[s.URL], filepath.Base(ruta))
		}
	}
	return usos, nil
}

// comprobarTodas pide las URLs de 6 en 6 a la vez: bastante más rápido que
// una a una, sin saturar a nadie.
func comprobarTodas(usos map[string][]string) []resultado {
	cliente := &http.Client{Timeout: 30 * time.Second}

	urls := make([]string, 0, len(usos))
	for u := range usos {
		urls = append(urls, u)
	}
	sort.Strings(urls) // salida siempre en el mismo orden

	resultados := make([]resultado, len(urls))
	turnos := make(chan struct{}, 6) // como mucho 6 peticiones a la vez
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			turnos <- struct{}{}
			defer func() { <-turnos }()
			estado, err := pedir(cliente, u)
			if err != nil || estado >= 500 {
				// Un fallo puntual de red o del servidor no es un enlace roto:
				// esperamos un poco y lo intentamos otra vez.
				time.Sleep(5 * time.Second)
				estado, err = pedir(cliente, u)
			}
			resultados[i] = resultado{url: u, estado: estado, err: err, usos: usos[u]}
		}()
	}
	wg.Wait()
	return resultados
}

// pedir hace un GET y devuelve el código HTTP. Usamos GET y no HEAD porque
// algunas webs responden mal a HEAD. Las redirecciones se siguen solas.
func pedir(cliente *http.Client, url string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	// Algunas webs rechazan peticiones sin un User-Agent de navegador.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; franc-academy-checklinks)")
	resp, err := cliente.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close() // no necesitamos el contenido, solo el código
	return resp.StatusCode, nil
}

// clasificar decide qué significa cada respuesta:
//   - 2xx: ok.
//   - 403 y 429: aviso. Suelen ser webs que bloquean robots o piden ir más
//     despacio; la página puede estar perfectamente. Conviene mirarla a mano.
//   - Resto (404, 410, 5xx tras reintentar, errores de red): roto.
func clasificar(r resultado) string {
	switch {
	case r.err == nil && r.estado >= 200 && r.estado < 300:
		return "ok"
	case r.estado == http.StatusForbidden || r.estado == http.StatusTooManyRequests:
		return "aviso"
	default:
		return "roto"
	}
}

func describir(r resultado) string {
	if r.err != nil {
		return "error: " + r.err.Error()
	}
	return fmt.Sprintf("HTTP %d", r.estado)
}
