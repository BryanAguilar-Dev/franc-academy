# franc-academy: instrucciones para agentes de IA

App de cultura general de Bryan, servida desde Franc (Debian, Docker,
Tailscale) en `127.0.0.1:8081`. El 8080 lo usa Finanzas: no tocarlo.

## Antes de trabajar
- Lee `README.md` (arquitectura) y `CONTENIDO.md` (reglas de las lecciones).
- Trabaja por fases y espera a que Bryan valide cada una antes de seguir.
- Explica el porqué de cada decisión en frases cortas. Bryan está aprendiendo Go.

## Código
- Go 1.24, solo librería estándar. Sin frameworks en la web.
- `go test ./...` debe pasar siempre: también valida el contenido.
- No añadas dependencias, login ni funciones que no se hayan pedido.

## Contenido
- Investiga en fuentes fiables antes de escribir y cita cada lección.
- No inventes datos, citas ni URLs: toda URL de `sources` debe haberse
  comprobado que existe.
- Publica por tandas pequeñas y revisadas. Calidad antes que cantidad.

## Commits
- Usa el correo `noreply` de GitHub de Bryan, nunca uno personal.
