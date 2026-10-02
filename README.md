# Order Tracking

Sistema de rastreamento de pedidos em Go. O cliente cria um pedido e acompanha a mudança de status (`CREATED → PAID → PICKING → SHIPPED → DELIVERED`, ou `CANCELED` antes do envio).

O projeto está sendo construído em etapas. Esta primeira entrega cobre a API REST, a persistência em PostgreSQL e a estrutura da aplicação com Uber Fx.

## Stack

- **Go 1.24+** com `net/http` (roteamento por método e path do Go 1.22)
- **PostgreSQL 16** via `pgx/v5`
- **goose** para migrações versionadas (embutidas no binário)
- **Uber Fx** para injeção de dependências e ciclo de vida
- **slog** para logs estruturados em JSON
- **Docker** e **Docker Compose**

## Arquitetura

Organização inspirada em DDD, com dependências apontando para dentro (o domínio não conhece banco nem HTTP):

```
cmd/order-api/              ponto de entrada (fx.New(...).Run())
internal/
  app/                      composição dos módulos Fx
  order/
    domain/                 agregado Order, máquina de estados, erros, interface Repository
    application/            casos de uso (criar, listar, mudar status, histórico)
    infra/postgres/         implementação do Repository com pgx
    infra/memory/           implementação em memória (usada nos testes)
    transport/http/         handlers REST e DTOs
  platform/
    config/                 configuração via variáveis de ambiente
    logger/                 slog JSON + integração com logs do Fx
    database/               pool pgx + migrações no OnStart
    httpserver/             servidor, middlewares (request ID, log, recover), health checks
    httpx/                  helpers de JSON e erro padronizado
  web/                      interface de teste (index.html embutido)
migrations/                 arquivos SQL do goose
```

### Decisões técnicas

- **Regras de negócio no domínio:** transições de status válidas ficam em `domain/status.go`. O handler HTTP e o banco só refletem o que o domínio decide.
- **Erros de domínio como sentinelas** (`ErrOrderNotFound`, `ErrInvalidTransition`...), propagados com `fmt.Errorf("...: %w", err)` e traduzidos para HTTP com `errors.Is` em um único lugar.
- **Concorrência otimista:** a atualização de status usa `WHERE id = $1 AND status = $2`. Se duas requisições tentarem mudar o mesmo pedido ao mesmo tempo, a segunda recebe `409 concurrent_update` em vez de sobrescrever a primeira.
- **Transações:** pedido, itens e histórico são gravados na mesma transação.
- **Sem N+1:** a listagem busca os itens de todos os pedidos numa única query (`WHERE order_id = ANY($1)`).
- **Datas em UTC com precisão de microssegundos**, igual ao `TIMESTAMPTZ` do Postgres, para que o valor em memória seja igual ao relido do banco.
- **Fx com grupos de rotas:** cada módulo registra suas rotas no grupo `routes`; o servidor HTTP não precisa conhecer os módulos.
- **Graceful shutdown:** ao receber SIGINT/SIGTERM, o Fx executa os `OnStop` em ordem inversa (primeiro o servidor HTTP para de aceitar requisições, depois o pool do banco é fechado).

## Endpoints

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/orders` | Cria um pedido |
| `GET` | `/orders` | Lista pedidos (`?status=&limit=&offset=`) |
| `GET` | `/orders/{id}` | Busca um pedido |
| `PATCH` | `/orders/{id}/status` | Muda o status |
| `GET` | `/orders/{id}/history` | Histórico de status |
| `GET` | `/` | Interface web de teste |
| `GET` | `/healthz` | Processo está vivo |
| `GET` | `/readyz` | Banco está acessível |

Erros seguem um formato único:

```json
{ "error": { "code": "invalid_transition", "message": "invalid status transition: PAID -> DELIVERED" } }
```

As respostas de pedido trazem `next_statuses`, a lista de transições permitidas, para que o cliente não precise duplicar a regra de negócio.

Exemplos prontos em [`api.http`](api.http).

## Interface de teste

Com a API rodando, acesse **http://localhost:8080/**. A página (HTML + JavaScript puro, embutida no binário com `go:embed`) permite criar pedidos, filtrar, mudar status, ver o histórico e mostra um log das requisições com status HTTP, tempo e `X-Request-ID`.

## Como rodar

Na primeira vez, gere o `go.sum`:

```bash
go mod tidy
```

### Tudo no Docker

```bash
docker compose up --build
```

A API sobe em `http://localhost:8080` e aplica as migrações automaticamente.

### API local + Postgres no Docker

```bash
docker compose up -d postgres
```

PowerShell:

```powershell
$env:DATABASE_URL="postgres://app:app@localhost:5432/orders?sslmode=disable"
go run ./cmd/order-api
```

Bash:

```bash
DATABASE_URL="postgres://app:app@localhost:5432/orders?sslmode=disable" go run ./cmd/order-api
```

## Testes

Testes unitários (domínio, casos de uso, handlers e grafo do Fx):

```bash
go test ./...
```

Testes de integração do repositório (precisam do Postgres do compose; usam o banco `orders_test`):

```powershell
$env:TEST_DATABASE_URL="postgres://app:app@localhost:5432/orders_test?sslmode=disable"
go test -tags=integration ./internal/order/infra/postgres/...
```

## Próximas etapas

- [ ] Publicação de eventos de status em **SNS** e consumo via **SQS** por um `order-worker` (LocalStack)
- [ ] Notificação em tempo real via **WebSocket** (`/ws/orders/{id}`)
- [ ] Upload de comprovante no **S3** com presigned URL
- [ ] Credenciais do banco no **Secrets Manager**
- [ ] Testes de integração com **testcontainers-go**
- [ ] Pipeline de CI (lint + testes)
