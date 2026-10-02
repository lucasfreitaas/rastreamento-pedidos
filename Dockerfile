# ---- build ----
FROM golang:1.24-alpine AS build
WORKDIR /src

# Copiar só go.mod/go.sum primeiro aproveita o cache do Docker: as
# dependências só são baixadas de novo quando esses arquivos mudam.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/order-api ./cmd/order-api

# ---- runtime ----
# Imagem distroless: sem shell e sem gerenciador de pacotes, roda como
# usuário não-root. Menor superfície de ataque e imagem bem pequena.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/order-api /order-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/order-api"]
