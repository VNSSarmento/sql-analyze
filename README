# SQL Query Performance Analyzer

Serviço backend em **Go** que monitora queries no PostgreSQL em tempo real e detecta comportamento anômalo com base no **histórico estatístico de cada query individual** — não em um limite fixo de milissegundos. Quando uma anomalia é detectada, um alerta é publicado de forma assíncrona e resiliente via **Redis Streams**.

![CI](https://github.com/VNSSarmento/sql-analyze/actions/workflows/ci.yml/badge.svg)
![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-blue)

---

## Por que esse projeto existe

Backend developer em transição, com **4+ anos de experiência prévia como analista de dados**, otimizando queries SQL em ambiente corporativo. Esse projeto não é um CRUD de portfólio — é a tentativa de transformar esse background real em um serviço de observabilidade de verdade: a pergunta que ele responde ("essa query está anormal *pra ela mesma*, considerando o próprio histórico?") é literalmente o julgamento que um analista de dados experiente faz manualmente, todos os dias, agora automatizado.

## O diferencial técnico

A maioria das ferramentas de monitoramento usa um **threshold fixo** ("alerta se > 500ms"). O problema: isso trata igual uma query que sempre foi rápida e desviou, e uma query que sempre foi instável e continua dentro do padrão dela.

Este projeto detecta anomalia **relativa ao histórico de cada query**, usando:

- **[Algoritmo de Welford](https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Welford's_online_algorithm)** — calcula média e variância de forma incremental, sem guardar histórico bruto de execuções (só 3 números por query: contagem, média, soma de diferenças quadráticas).
- **Z-score** — cada nova execução é comparada contra a distribuição histórica daquela query específica. Alerta quando o desvio ultrapassa 3 desvios padrão.
- **Variância amostral (correção de Bessel, `n-1`)** — calibrada para o `n_min` baixo (8 execuções) que o projeto usa, priorizando detectar anomalias cedo em queries de baixa frequência (analíticas, 10-15 execuções/semana) em vez de esperar semanas por uma amostra "estatisticamente confortável".

---

## Arquitetura

Clean Architecture / Ports & Adapters, com um **domain model rico**: a lógica de detecção de anomalia vive como método da própria entidade (`Query.RegisterExecution`), não espalhada em serviços.

```
├── cmd/api/main.go          → composition root: monta dependências, inicia servidor
├── internal/
│   ├── domain/                → entidades + interfaces (portas). Zero import de infraestrutura.
│   ├── usecase/                → regras de negócio, orquestra via interfaces do domain
│   ├── adapter/
│   │   ├── http/                 → handlers REST (Gin)
│   │   ├── postgres/              → implementa os repositórios com SQL real
│   │   ├── redis/                  → cache (cache-aside) + mensageria (Streams)
│   │   └── worker/                  → coletor de métricas + consumidor de alertas
│   └── config/                        → conexões (Postgres pool, Redis client)
├── migrations/                          → schema versionado (golang-migrate)
├── deployments/                          → Dockerfile (multi-stage) + docker-compose.yml
└── .github/workflows/ci.yml               → lint → testes → build da imagem
```

**Regra seguida em todo o projeto:** `domain/` nunca importa `database/sql`, `net/http` ou qualquer lib de infraestrutura — só entidades e contratos (interfaces). Cada `adapter/` implementa um contrato, escondendo o detalhe técnico (SQL, comandos Redis, JSON) atrás dele. Trocar Postgres por outro banco, por exemplo, significaria reescrever um adapter — o `domain` e os `usecase` não mudariam uma linha.

---

## Stack

| Categoria | Tecnologia |
|---|---|
| Linguagem | Go 1.26 |
| API REST | [Gin](https://github.com/gin-gonic/gin) |
| Banco relacional | PostgreSQL 15 (`pgx/v5`, pool de conexões) |
| Cache & Mensageria | Redis 7 (`go-redis/v9`) — cache-aside + Streams com consumer groups |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Testes | `testing` + `testify` (asserts e mocks) |
| Lint | `golangci-lint` v2 |
| Containerização | Docker (multi-stage build) + Docker Compose |
| CI/CD | GitHub Actions |

---

## Funcionalidades

### API REST

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/queries/slowest?limit=10` | Top N queries com maior tempo médio (cache-aside, TTL 1min) |
| `GET` | `/queries/:queryID/:dbUser` | Detalhe de uma query específica, por identidade composta |

**Exemplo de resposta** (`GET /queries/slowest?limit=1`):
```json
[
  {
    "query_id": "-3936638199876867620",
    "db_user": "admin",
    "normalized_query": "SELECT * FROM orders WHERE customer_id = $1",
    "executions_count": 142,
    "mean_time_ms": 12.45,
    "m2": 87.3,
    "last_execution_at": "2026-08-20T21:31:03Z",
    "last_anomaly_at": "2026-08-20T21:31:03Z",
    "created_at": "2026-08-15T15:52:08Z"
  }
]
```

### Worker de coleta (`adapter/worker.Collector`)

Lê `pg_stat_statements` periodicamente. Como essa view só expõe **agregados cumulativos** (não execuções individuais), o coletor mantém um snapshot em memória por query e calcula o **delta** entre leituras — o tempo médio "desse intervalo" é derivado daí, não lido diretamente.

### Pipeline de alertas (Redis Streams)

```
Anomalia detectada → XADD (produtor)
                          │
                          ▼
                  Stream: queries:alertas
                          │
          ┌───────────────┴───────────────┐
          ▼                                ▼
  Consumo contínuo                 Reclaim de mensagens
  (XREADGROUP)                     travadas a cada 1min
                                    (XAUTOCLAIM, idle > 3min)
          │                                │
          └───────────────┬────────────────┘
                           ▼
                    processMessage
                (resolve contato, loga, XACK)
```

**Por que Streams em vez de Pub/Sub:** Pub/Sub perde mensagem se ninguém estiver ouvindo no momento exato da publicação. Streams persiste até confirmação (`XACK`) — essencial para não perder alerta se o consumidor cair no meio do processamento. O `XAUTOCLAIM` garante que nenhuma mensagem fica presa para sempre em um consumidor travado.

---

## Como rodar localmente

Pré-requisitos: Docker e Docker Compose.

```bash
# 1. Clonar e configurar
git clone https://github.com/VNSSarmento/sql-analyze.git
cd sql-analyze
cp .env.example .env

# 2. Subir a infraestrutura (Postgres + Redis)
docker compose -f deployments/docker-compose.yml up -d db redis

# 3. Aplicar as migrations
docker compose -f deployments/docker-compose.yml up -d migrate

# 4. Ativar a extensão pg_stat_statements (necessária para o coletor)
docker exec -it postgres-sql_analyze psql -U admin -d sql_analyze \
  -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"

# 5. Subir a API
docker compose -f deployments/docker-compose.yml up -d --build api

# 6. Conferir
curl http://localhost:3000/health
```

## Testes

```bash
go test -v -race ./...
```

- **Domain** (`Query.RegisterExecution`): table-driven tests puros, sem infraestrutura — cobrem cold start, execução normal, anomalia, e o caso especial de desvio padrão zero.
- **Usecases**: mocks via `testify/mock` para as interfaces do domain (`QueryRepository`, `AlertPublisher`, `QueryCache`) — cobrem os caminhos de sucesso, falha e degradação (ex: cache indisponível não deveria derrubar uma leitura).
- `-race` ativo — o projeto usa `sync.WaitGroup` e múltiplas goroutines (coletor, consumidor de alertas em duas goroutines paralelas), então detecção de data race é relevante, não decorativo.

## CI/CD

Pipeline no GitHub Actions com três jobs (`lint` e `test` em paralelo, `build` dependente dos dois):

1. **Lint** — `golangci-lint` (errcheck, revive, staticcheck, govet, unused)
2. **Test** — `go test -race ./...`
3. **Build** — valida que a imagem Docker (multi-stage) builda com sucesso

---

## Decisões técnicas relevantes

Escolhas conscientes, com o trade-off que cada uma assume:

- **`n_min = 8`** (mínimo de execuções antes de começar a detectar anomalia) — calibrado para queries analíticas esporádicas. Aceita menos confiabilidade estatística inicial em troca de não ficar semanas "cego" em queries de baixa frequência.
- **Sem throttle de alertas** — dispara notificação a cada execução anômala, mesmo repetida. Em ambiente com múltiplos problemas concorrentes, um alerta único poderia passar despercebido antes de ser resolvido.
- **Cache com graceful degradation** — se o Redis cair, as leituras degradam para consultar o Postgres diretamente, em vez de falhar. Cache nunca é ponto único de falha para uma leitura.
- **Identidade de `Query` é o par `(query_id, db_user)`**, não um ID técnico — porque cada analista tem conta individual no Postgres, e `pg_stat_statements` já separa estatísticas por usuário nativamente.

## Limitações conhecidas

Documentadas conscientemente, não esquecidas:

- **Snapshots do coletor vivem em memória** — um restart da aplicação perde o baseline de comparação; a primeira leitura de cada query pós-restart não gera análise.
- **Ruído de auto-referência** — o próprio coletor e o `Save` do repositório aparecem em `pg_stat_statements`; filtrados por texto da query (solução pragmática, não a ideal — a robusta seria um usuário técnico dedicado, com filtro por role).
- **Notificação de alerta é só log estruturado** — o contato do usuário é resolvido (tabela `user_contacts`, seed manual), mas o envio real (e-mail/Slack) ainda não está implementado.
- **`ConsumerName` do consumidor de alertas é fixo** — funciona com uma única instância da aplicação; escalar para múltiplas réplicas exigiria um nome único por processo (ex: hostname + PID).

## Roadmap

- [ ] Envio real de notificação (SMTP ou webhook Slack)
- [ ] Testes de integração com Postgres real (`testcontainers-go`)
- [ ] Endpoint de cadastro de contatos (hoje é seed manual via SQL)
- [ ] Filtro de auto-referência por role dedicado, em vez de por texto da query

---

## Autor

**Vinicius Sarmento Ramos** — Backend Developer (Go, PHP) | Background em análise de dados e otimização de SQL

[LinkedIn](https://linkedin.com/in/vinicius-sarmento-ramosa1b7432a4) · [GitHub](https://github.com/VNSSarmento)