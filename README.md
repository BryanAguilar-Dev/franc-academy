# Franc Academy

Plataforma gratuita de cultura general. Contenido propio con fuentes, tests
después de cada lección, exámenes por categoría y reto diario. Funciona sin
conexión a ninguna IA ni suscripción: todo el contenido está en este
repositorio.

Forma parte de [Aprende con Franc](https://github.com/BryanAguilar-Dev/aprende)
y se sirve desde Franc, el servidor de casa, a través de Tailscale.

## Estado del catálogo

| Categoría | Lecciones |
|-----------|-----------|
| Universo y astronomía | 7 de 15 (categoría piloto) |
| Ciencia y naturaleza | en preparación |
| Historia y civilizaciones | en preparación |
| Cerebro y psicología | en preparación |
| Tecnología e IA | en preparación |
| Cultura general | en preparación |
| Programación en Go | 6 (bloque 1 en curso) |

Las lecciones se publican por tandas, revisadas y con sus fuentes. Preferimos
pocas y buenas antes que muchas y superficiales.

## Cómo funciona

```
iPhone / PC ──Tailscale──► Franc 127.0.0.1:8081
                            └─ contenedor franc-academy
                               └─ binario Go (librería estándar)
                                  ├─ web/        interfaz (HTML, CSS, JS)
                                  ├─ content/    lecciones en JSON
                                  └─ /data/progress.json   tu progreso
```

- **Contenido separado de la interfaz.** Cada lección es un archivo JSON en
  `content/<categoría>/`. Añadir una lección no requiere tocar código.
- **El contenido se valida con tests.** `go test ./...` comprueba que cada
  lección tiene todos sus apartados, exactamente 10 preguntas con una única
  respuesta correcta, explicación en cada opción y al menos 2 fuentes https.
  Se ejecutan en cada pull request y en cada push a main
  (`.github/workflows/tests.yml`), y si algo falla, la imagen Docker no se
  construye. Ojo: los tests comprueban la estructura, no que la explicación
  sea correcta. Eso se revisa a mano en cada PR, con las fuentes delante.
- **El progreso vive en Franc**, en un volumen Docker. Es el mismo desde
  cualquier dispositivo. El navegador solo envía eventos ("test hecho: 8/10")
  y el servidor decide cómo cambia el progreso.
- **Sin login.** Solo se accede por Tailscale. Si algún día se publica en
  Internet, hay que añadir autenticación antes.

| Carpeta | Qué hay |
|---------|---------|
| `cmd/academy/` | `main.go`: arranca el servidor |
| `cmd/checklinks/` | Comprueba que las URLs de las fuentes siguen vivas (lo lanza cada lunes `.github/workflows/enlaces.yml`) |
| `internal/content/` | Carga y valida las lecciones |
| `internal/progress/` | Guarda el progreso en JSON (escritura atómica) |
| `internal/server/` | Rutas HTTP y API |
| `content/` | Categorías, lecciones e ilustraciones SVG |
| `web/` | Interfaz |

## Probar en local (necesita Go 1.24)

```bash
go test ./...                 # valida código y contenido
go run ./cmd/academy          # http://localhost:8081, progreso en ./data
```

## Desplegar en Franc

```bash
git clone https://github.com/BryanAguilar-Dev/franc-academy.git ~/franc-academy
cd ~/franc-academy
docker compose up -d --build
docker compose logs          # debe decir "escuchando en :8081"
curl http://127.0.0.1:8081/healthz   # debe responder: ok
```

Después se publica en la tailnet igual que Finanzas (ver
`sudo tailscale serve status` para ver cómo está Finanzas).

**Actualizar** tras nuevas lecciones: `git pull && docker compose up -d --build`.
El progreso no se pierde: está en el volumen `academy-data`.

**Copia de seguridad del progreso:**

```bash
docker cp franc-academy:/data/progress.json ~/backups/academy-$(date +%F).json
```

## Añadir una lección

Ver [`CONTENIDO.md`](CONTENIDO.md).

## Licencia del contenido

Los textos son originales, redactados a partir de las fuentes citadas en cada
lección (NASA, ESA, OpenStax y otras). No se copian textos de esas fuentes.
