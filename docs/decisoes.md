# Guia de referência — SQL Query Performance Analyzer

Material de consulta para continuar o desenvolvimento sozinho. Não contém código Go pronto (isso é pra você escrever) — só decisões, comandos, e o "porquê" de cada escolha, pra você não precisar reconstruir o raciocínio do zero.

---

## 1. Visão geral do projeto

Serviço em Go que conecta num Postgres via `pg_stat_statements`, detecta queries com comportamento anômalo (não só "lentas por um número fixo", mas por desvio do próprio histórico da query), expõe API REST, e usa Redis Streams para alertar de forma assíncrona e resiliente a falhas.

**Objetivo do projeto:** fechar a lacuna de portfólio Go público, já que o projeto real (Campaign Manager) é trabalho de cliente sob NDA e não pode ser mostrado. Esse projeto reaproveita seu background de 4+ anos em análise de dados/SQL como diferencial real, não fictício.

---

## 2. Arquitetura (Hexagonal / Ports & Adapters)

```
seu-projeto/
├── cmd/api/main.go          → composition root: monta dependências, inicia servidor
├── internal/
│   ├── domain/               → entidades + interfaces (portas). Zero import de infra.
│   ├── usecase/               → regras de negócio, recebe interfaces do domain
│   ├── adapter/
│   │   ├── http/               → handlers HTTP (Gin)
│   │   ├── postgres/            → implementa as interfaces do domain com SQL real
│   │   ├── redis/                → implementa cache + streams
│   │   └── worker/                → coletor + consumidor de alertas
│   └── config/
│       ├── postgres.go            → conexão + pool Postgres
│       └── redis.go                → conexão client Redis
├── migrations/                      → arquivos .up.sql / .down.sql
├── deployments/
│   ├── Dockerfile
│   └── docker-compose.yml
├── .github/workflows/ci.yml
├── .env / .env.example
└── go.mod
```

**Regra de ouro:** `domain/` nunca importa `database/sql`, `net/http`, `redis`, nada de infraestrutura. Só entidades e interfaces (contratos). Quem implementa esses contratos são os `adapter/`.

**Princípio Go aplicado:** "accept interfaces, return concrete structs" — construtores (`NewX`) devolvem structs concretos; funções que recebem dependências recebem interfaces, não implementações.

---

## 3. Docker Compose — padrões estabelecidos

### Checklist de cada serviço
- [ ] `image` com versão fixa, nunca `latest`
- [ ] `healthcheck` definido (não confiar só em `depends_on` simples)
- [ ] `depends_on` usando `condition: service_healthy` (serviços contínuos) ou `condition: service_completed_successfully` (serviços que rodam e terminam, como `migrate`)
- [ ] `env_file: - ../.env` como fonte única — nunca duplicar valores hardcoded em vários lugares

### As 3 condições de `depends_on`
| Condição | Quando usar |
|---|---|
| `service_started` (padrão) | Só espera o container iniciar — raramente suficiente |
| `service_healthy` | Espera passar no healthcheck — usar pra `db`, `redis` |
| `service_completed_successfully` | Espera o container terminar com exit 0 — usar pra `migrate` |

### Pegadinha do `${...}` vs `$$...` no compose
- `${VAR}` no `command:`/`environment:` do YAML → resolvido pelo **Compose**, no momento de ler o arquivo, usando o ambiente do host/shell. Se a variável só existe via `env_file` (dentro do container), isso resolve **vazio**, silenciosamente.
- `$$VAR` dentro de um `command` executado via `sh -c` → resolvido pelo **shell dentro do container**, em tempo de execução, lendo do ambiente real do processo (que já recebeu o `env_file`).
- Regra prática: se a variável vem de `env_file`, use `$$` + `entrypoint: ["sh", "-c"]`.

### Serviço `migrate` (decisão: serviço separado no compose, não embed no binário)
Motivo: mantém a aplicação sem responsabilidade de gerenciar schema, evita dependência extra no `go.mod`, permite rodar migration isolada em CI sem subir a API inteira.

Estrutura do serviço (referência, não copiar sem entender):
- `image: migrate/migrate`
- `entrypoint: ["sh", "-c"]` + `command` como string única usando `$$POSTGRES_USER` etc.
- `depends_on: db: condition: service_healthy`
- `restart: "no"`

### Comandos migrate úteis
```bash
# Criar uma nova migration (gera .up.sql e .down.sql)
docker run --rm -v $(pwd)/migrations:/migrations migrate/migrate create -ext sql -dir /migrations -seq nome_da_migration

# Aplicar migrations pendentes
docker compose up -d migrate

# Ver status/logs
docker compose logs migrate
```

---

## 4. Config package — armadilhas já identificadas

Ao escrever `postgres.go` / `redis.go`, os erros mais comuns que já caímos:

1. **Campo do struct precisa ser ponteiro** (`*pgxpool.Pool`, `*redis.Client`), nunca valor — copiar por valor quebra mutexes internos.
2. **`%d` com string do `os.Getenv` é bug silencioso** — `os.Getenv` sempre devolve `string`; usar `%s` sempre na `Sprintf`, mesmo pra "número" como porta.
3. **`os.Getenv("NOME_DA_VARIAVEL")` recebe a chave, não o valor** — erro comum é passar o valor esperado por engano.
4. **Campo minúsculo = não exportado** — se outro pacote precisa acessar, usar maiúscula (decisão tomada: campo público direto, sem método getter, por simplicidade nesse estágio do projeto).
5. **`godotenv.Load()` não deve ser fatal** — dentro do container, `.env` não existe (variáveis vêm do `env_file` do compose). Tratar como aviso (`log.Println`), não `log.Fatal`.
6. **`pgxpool.New()` e `redis.NewClient()` são lazy** — não conectam de verdade sozinhos. Sempre chamar `.Ping(ctx)` explicitamente depois de criar.
7. **`redis.Client.Ping(ctx)` devolve `*redis.StatusCmd`, não `error`** — precisa chamar `.Err()` no resultado: `client.Ping(ctx).Err()`.
8. **`go-redis` — atenção à versão da lib.** Usar sempre `github.com/redis/go-redis/v9` (não a antiga `github.com/go-redis/redis`, que não usa `context.Context`). Se aparecerem as duas no `go.mod`, rodar `go mod tidy` depois de corrigir os imports.
9. **`go-redis` não separa host/porta** — usa `Addr: "host:porta"` concatenado manualmente via `fmt.Sprintf`, diferente do pgx que recebe tudo numa DSN só.

---

## 5. Comandos Redis — cheatsheet por caso de uso

### Cache (endpoint `GET /queries/slowest`)
| Comando Go (client) | O que faz |
|---|---|
| `Set(ctx, key, value, expiration)` | Grava com TTL embutido |
| `Get(ctx, key)` | Busca — erro `redis.Nil` quando não existe (tratar separado de erro de conexão) |
| `Del(ctx, keys...)` | Remove manualmente |
| `Exists(ctx, keys...)` | Checa existência sem trazer valor |

### Streams (mensageria de alertas) — testar sempre via `redis-cli` antes de codar
```bash
docker exec -it redis-sql_analyze redis-cli
```

```
# Criar stream + consumer group (uma vez)
XGROUP CREATE queries:alertas workers-alertas $ MKSTREAM

# Produtor publica evento
XADD queries:alertas * query_id 42 tempo_ms 850 threshold_ms 500

# Consumidor lê (bloqueia até 5s esperando mensagem nova)
XREADGROUP GROUP workers-alertas worker-1 COUNT 1 BLOCK 5000 STREAMS queries:alertas >

# Confirma processamento
XACK queries:alertas workers-alertas <id-da-mensagem>

# Lista mensagens pendentes (não confirmadas)
XPENDING queries:alertas workers-alertas

# Reatribui mensagem travada para outro worker
XCLAIM queries:alertas workers-alertas worker-2 60000 <id-da-mensagem>
```

**Por que Streams e não Pub/Sub:** Pub/Sub perde mensagem se ninguém estiver ouvindo no momento exato do envio. Streams persiste a mensagem até alguém confirmar com `XACK` — essencial pra não perder alerta se o worker cair.

**Responsabilidade separada:** o "faxineiro" que roda `XPENDING` + `XCLAIM` periodicamente é um processo distinto do consumidor principal — não roda sozinho automaticamente.

### O que ignorar por enquanto
Pub/Sub, hashes (`HSet`), listas (`LPush`), sets (`SAdd`), sorted sets (`ZAdd`), transações (`TxPipeline`), scripting Lua (`Eval`) — nenhum tem uso claro no escopo atual.

---

## 6. Decisões de domínio — entidade `Query`

### Por que anomalia relativa, não threshold fixo
Um threshold fixo (ex: "alerta se > 500ms") trata igual uma query que sempre foi rápida e desviou, e uma query que sempre foi instável e continua dentro do padrão dela. Detecção relativa ao histórico da própria query resolve isso — e evita pedir ao usuário leigo um número de milissegundos que ele não sabe julgar.

### Algoritmo: Welford's online + z-score
- Atualiza média (`meanMs`) e variância (via `m2`) de forma incremental, **sem guardar histórico bruto de execuções** — só 3 números por query: `n`, `meanMs`, `m2`.
- Para cada nova execução: **primeiro calcula o z-score contra o estado anterior**, decide se é anomalia, **só depois** atualiza as estatísticas com o novo valor. (Se atualizasse antes, a execução "amorteceria" a própria detecção.)

### Parâmetros fechados
| Parâmetro | Valor | Motivo |
|---|---|---|
| `n_min` (cold start) | **8** execuções | Calibrado para queries analíticas esporádicas (10-15x/semana, incluindo queries longas com CTEs de IA) — equilíbrio entre confiabilidade estatística e não ficar semanas "cego" |
| Throttle de alertas | **Nenhum** — dispara toda vez que anômalo | Ambiente com múltiplos problemas concorrentes; alerta único poderia cair no esquecimento antes de resolver |
| Toda execução conta para estatística | Sim, sem piso mínimo de tempo | Decisão explícita — nenhuma execução é ignorada |

**Limitação conhecida, documentada conscientemente:** com `n_min` baixo, o desvio padrão inicial é menos confiável — o projeto prioriza detectar cedo, aceitando mais ruído nas primeiras execuções.

### Identidade da entidade
Chave é o par **(query_id, db_user)**, não só `query_id` — porque cada analista tem conta individual no Postgres, e `pg_stat_statements` já separa estatísticas por usuário nativamente. Isso também deixa as estatísticas mais precisas (não mistura o padrão de execução de pessoas diferentes).

### Schema aplicado (migration `000001_create_queries_table`)
Campos: `id`, `query_id`, `db_user`, `normalized_query`, `executions_count`, `mean_time_ms`, `m2`, `last_execution_at`, `last_anomaly_at`, `created_at`, com `UNIQUE (query_id, db_user)`.

### Em aberto — ainda não modelado
- Tabela/mapeamento de **usuário do banco → contato real** (e-mail/Slack), pra notificar quem ainda não otimizou a query. `pg_stat_statements` dá o usuário do Postgres, não o contato — isso precisa de cadastro próprio.
- Interface `QueryRepository` (métodos que o adapter Postgres vai implementar).
- Entidade/config de alerta (se threshold vira relativo por completo, ou convive com algum valor absoluto de fallback).

---

## 7. Roteiro do que falta (ordem sugerida)

1. ~~`main.go` mínimo (builda)~~ ✅
2. ~~`config/postgres.go` + Ping~~ ✅
3. ~~`config/redis.go` + Ping~~ ✅
4. ~~Servidor HTTP vivo (`/health`)~~ ✅
5. ~~Entidade `Query` desenhada conceitualmente~~ ✅
6. ~~Migration da tabela `queries`~~ ✅
7. ~~Interface `QueryRepository` (contrato, sem implementação)~~✅
8. Tabela + mapeamento usuário → contato
9. ~~`usecase/` — orquestra domain + repository~~
10. ~~`adapter/postgres/` — implementação real do repository~~
11. `adapter/http/` — handlers Gin ligando rotas aos usecases
12. ~~`adapter/redis/` — cache do endpoint + produtor do stream~~
13. `adapter/worker/` — coletor (lê `pg_stat_statements`) + consumidor de alertas (`XREADGROUP`) + "faxineiro" de pendências (`XPENDING`/`XCLAIM`)
14. Testes unitários (domain/usecase) + integração (`testcontainers-go` para Postgres)
15. `golangci-lint` configurado
16. ~~`.github/workflows/ci.yml` — lint → testes → build da imagem Docker~~
17. Graceful shutdown (`SIGTERM`/`SIGINT` fechando pool/client antes de encerrar)
18. README com diagrama de arquitetura + decisões documentadas (reaproveitar as justificativas deste guia)

---

## 8. Comandos do dia a dia

```bash
# Subir tudo
docker compose up -d

# Subir só alguns serviços
docker compose up -d db redis

# Forçar rebuild da API depois de mudar código
docker compose up -d --build api

# Subir um serviço sem religar dependências (útil pra testar falha isolada)
docker compose up -d --build --no-deps api

# Ver status, incluindo containers parados
docker compose ps -a

# Logs de um serviço específico
docker compose logs api

# Ver logs em tempo real (sem -d, direto no terminal)
docker compose up --build api

# Entrar no Postgres
docker exec -it postgres-sql_analyze psql -U admin -d sql_analyze

# Entrar no Redis
docker exec -it redis-sql_analyze redis-cli
```

---

## 9. Troubleshooting — lições aprendidas

### Caso: migration reportava sucesso, mas a tabela não existia

**Sintoma:** `docker compose logs migrate` mostrava `1/u create_queries_table (Xms)`, a tabela `schema_migrations` tinha `version=1, dirty=false` — todo sinal de sucesso — mas `\dt` não listava a tabela `queries`.

**Cadeia de causas reais, na ordem em que aconteceram:**

1. **Permissão de arquivo no WSL.** O `docker run ... migrate/migrate create ...` roda como `root` dentro do container por padrão. Como o volume monta a pasta local do WSL, os arquivos `.up.sql`/`.down.sql` nasceram com dono `root`, impedindo salvar o conteúdo pelo editor local até rodar `sudo chown -R $USER:$USER migrations`.
2. **Migration rodou vazia antes da correção de permissão.** O `migrate up` já tinha sido executado com o `.up.sql` vazio (ou inacessível) — um script SQL vazio é *válido* pro Postgres, então o `migrate` marcou sucesso e versão 1 aplicada, mesmo sem criar nada.
3. **CREATE e DROP misturados no mesmo arquivo.** Numa tentativa de correção, o conteúdo de `up.sql` e `down.sql` acabou colado invertido/junto — o `up.sql` chegou a conter tanto `CREATE TABLE` quanto `DROP TABLE`. Resultado: criava e derrubava a tabela na mesma execução, sem erro nenhum.
4. **`entrypoint: ["sh", "-c"]` quebra comandos avulsos.** Como o compose usa `sh -c` pra resolver as variáveis `$$POSTGRES_USER`, rodar `docker compose run migrate <flags soltas>` faz o Compose repassar as flags pro `sh -c` como se fossem opções do próprio shell — gerando `illegal option -p`. **Correção:** usar `docker compose run --rm --entrypoint migrate migrate <flags>`, sobrescrevendo o entrypoint pra ir direto no binário `migrate`, sem passar pelo shell.
5. **`force 0` não significa "nada aplicado".** No `golang-migrate`, o estado "nenhuma migration aplicada" é a versão **`-1`** (`NilVersion`), não `0`. Rodar `force 0` registra a versão 0 como se fosse uma migration real — como não existe arquivo `000000_*`, qualquer tentativa seguinte de `up`/`down` falha com `file does not exist`. O comando certo seria `force -1`.
6. **Resolução final aplicada:** apagar manualmente as linhas da tabela `schema_migrations` (`DELETE FROM schema_migrations;` ou truncar) também reseta o estado de controle — mais direto que lidar com `force -1` nesse caso.

**Lições gerais, reaproveitáveis em qualquer debug de migration:**

- **O log do container `migrate` acumula entre execuções**, porque `restart: "no"` não remove o container — ele só para. Rodar `docker compose logs migrate` depois de várias tentativas mistura saídas de execuções diferentes. Sempre `docker compose rm -f migrate` antes de rodar de novo se quiser um log limpo, ou usar `docker logs <container>` direto pra conferir o estado mais recente.
- **Não confiar só no log — checar o estado real do banco.** `SELECT * FROM schema_migrations;` e `\dt` são a fonte da verdade; o log pode enganar (sucesso reportado ≠ efeito esperado no schema).
- **Gerar migrations já com o usuário certo**, evitando o problema de permissão desde a origem:
  ```bash
  docker run --rm --user $(id -u):$(id -g) -v $(pwd)/migrations:/migrations migrate/migrate create -ext sql -dir /migrations -seq nome_da_migration
  ```
- **Comandos avulsos do `migrate` (force, down, version) sempre com `--entrypoint migrate`** quando o serviço usa `sh -c` no compose:
  ```bash
  docker compose run --rm --entrypoint migrate migrate -path=/migrations -database "..." force -1
  ```

---

*Documento gerado a partir das decisões tomadas em conversa com Claude — sirva como ponto de partida, não como verdade absoluta. Se uma decisão aqui parar de fazer sentido conforme o projeto evolui, documente o porquê da mudança também.*