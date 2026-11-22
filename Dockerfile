# Etapa 1: compilar. Usa la imagen oficial de Go solo para construir.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
# Los tests validan el contenido: si una lección está mal, la imagen no se construye.
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /academy ./cmd/academy
RUN mkdir /data

# Etapa 2: ejecutar. Imagen mínima, sin shell, con un usuario sin privilegios.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /academy /academy
COPY --from=build --chown=nonroot:nonroot /data /data
ENV ADDR=:8081 DATA_DIR=/data TZ=Europe/Madrid
EXPOSE 8081
USER nonroot
ENTRYPOINT ["/academy"]
