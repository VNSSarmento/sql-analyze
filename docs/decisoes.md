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
- **Variância usa `n-1` (amostral, correção de Bessel)**, não `n` (populacional) — decisão consciente por causa do `n_min=8` baixo: com poucas amostras, a correção evita subestimar a variância real. **Atenção:** esse `-1` entra **só** no cálculo da variância pro z-score. A atualização da média do Welford's continua dividindo por `n` (o contador já incrementado) — são fórmulas independentes; usar `n-1` na média por engano quebra o algoritmo silenciosamente (bug real que já caímos aqui).
- **Caso especial: desvio padrão = 0** (todas as execuções anteriores foram idênticas). Sem tratamento, isso "mascara" a anomalia mais óbvia possível (query que sempre foi estável e mudou). Tratamento adotado: se `desvP == 0` e a nova execução for diferente da média, marca anomalia direto, sem calcular z-score (não existe z-score matematicamente válido sem desvio padrão — fica `0` nesse caso específico, é uma limitação documentada, não um bug).

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

### Payload do alerta (`AnomalyAlert`)
Campos fechados: `QueryID`, `DBUser`, `CurrentTimeMs`, `MeanTimeMs` (adicionado depois — permite ao worker consumidor montar mensagem tipo "rodou em Xms, quando a média é Yms" sem precisar consultar o banco de novo), `ZScore`, `DetectedAt`.

**Cuidado de nome:** usar `DBUser` (maiúsculo em "DB", seguindo convenção Go de siglas) em **todas** as structs do domínio — já caímos na inconsistência de ter `DbUser` numa struct e `DBUser` em outra.

### Rich Domain Model — o nome correto pro padrão aplicado
O projeto usa **Clean/Hexagonal Architecture** com um **rich domain model** (regra de negócio como método da entidade — `Query.RegisterExecution`), não "DDD completo". DDD é um guarda-chuva maior (ubiquitous language, bounded contexts, aggregates, value objects, domain events) que o projeto não implementa formalmente, e não precisa.

### Em aberto — ainda não modelado
- Tabela/mapeamento de **usuário do banco → contato real** (e-mail/Slack), pra notificar quem ainda não otimizou a query. `pg_stat_statements` dá o usuário do Postgres, não o contato — isso precisa de cadastro próprio.
- Entidade/config de alerta (se threshold vira relativo por completo, ou convive com algum valor absoluto de fallback) — hoje o threshold é 100% relativo (z-score), sem fallback absoluto implementado.

---

## 7. Camadas implementadas — usecase e adapters

### `usecase.AnalyzeQueryUseCase.Execute` — orquestração
Recebe `queryID`, `dbUser`, `normalizedQuery`, `executionTimeMs`. Fluxo: busca a query (`GetByID`) → se `ErrQueryNotFound`, monta uma nova → chama `query.RegisterExecution(executionTimeMs)` (todo o cálculo mora aí, o usecase não conhece Welford's nem z-score) → se `result.IsAnomaly`, monta `AnomalyAlert` e publica (erro do `Publish` só loga, não aborta — decisão consciente, perder um alerta pontual é menos grave que perder o dado estatístico) → `Save` no repository (esse erro sim aborta o `Execute`).

### `adapter/postgres.PostgresQueryRepository` — implementação real do `QueryRepository`
- **Upsert com `ON CONFLICT (query_id, db_user) DO UPDATE`** — evita condição de corrida entre "checar se existe" e "decidir inserir vs atualizar"; uma operação atômica só.
- **`Scan` exige ponteiros** (`&query.Campo`, não `query.Campo`) — passar valor em vez de endereço compila mas falha em runtime, silenciosamente, com erro genérico (não trava em tempo de compilação porque `Scan` aceita `...any`).
- **Contagem de colunas do SELECT precisa bater exatamente com a contagem de destinos do Scan** — o `id` (BIGSERIAL, chave técnica) foi tirado do SELECT porque o domínio não carrega esse campo (identidade real é `query_id`+`db_user`).
- **`pgx.ErrNoRows` → traduzido pra `domain.ErrQueryNotFound`** via `errors.Is` — é assim que o usecase distingue "não existe ainda" de "erro de banco de verdade".
- **Campos `*time.Time` nullable** (`LastAnomalyAt`) recebem `&query.Campo` normalmente — o `pgx` aloca automaticamente se não for `NULL`, deixa `nil` se for.
- **Sempre checar `rows.Err()` depois do loop `for rows.Next()`** — `Next()` devolve `false` tanto por "acabaram as linhas" quanto por "erro no meio do streaming"; sem o `rows.Err()` esses dois casos ficam indistinguíveis.

### `adapter/redis.StreamAlertPublisher` — implementação do `AlertPublisher`
- `Values` do `redis.XAddArgs` aceita `map[string]any` (o `any` é alias de `interface{}`, mesma coisa, mais idiomático desde Go 1.18).
- Timestamps (`DetectedAt`) precisam ser convertidos pra string antes de entrar no map — Redis não serializa `time.Time` sozinho. Usar `time.RFC3339` (data + hora + timezone), nunca um formato só-data — sem throttle de alertas, vários alertas da mesma query no mesmo dia ficariam indistinguíveis com formato só-data.
- **`EnsureConsumerGroup`** (roda uma vez, no `main.go`, na subida): usa `XGroupCreateMkStream` (não `XGroupCreate` simples) — o sufixo `MkStream` cria o stream também, caso ainda não exista nenhuma mensagem publicada. Erro `BUSYGROUP` (grupo já existe, de uma subida anterior) precisa virar `nil` no `return` — tratar como erro de verdade quebraria a aplicação em todo restart depois do primeiro.

### `adapter/redis.QueryCacheAdapter` — implementação do `QueryCache`
- Padrão **cache-aside**: leitura pergunta ao cache primeiro; se `miss`, busca no Postgres e popula o cache antes de devolver.
- Chave inclui o `limit` (`queries:slowest:10` ≠ `queries:slowest:50`) — senão pedidos com limites diferentes se sobrescrevem.
- Serialização via `json.Marshal`/`json.Unmarshal` — Redis só entende texto/bytes, não structs Go.
- `redis.Nil` → traduzido pra `domain.ErrCacheMiss` (erro sentinela próprio, paralelo ao `ErrQueryNotFound`) — cache miss é rotina, não deveria se misturar com "Redis está fora do ar".
- TTL curto (1 minuto) — o coletor atualiza os dados constantemente por trás; cache "velho" mostraria números defasados.

### `usecase.ListSlowestQueriesUseCase.Execute` — orquestra o cache-aside
- Tenta `cache.GetSlowest` primeiro. Se `err == nil`, devolve direto — nem toca no Postgres.
- **Graceful degradation:** tanto `ErrCacheMiss` quanto qualquer outro erro do Redis (indisponível, timeout) caem no mesmo caminho — busca no Postgres. A diferença é só que erro "de verdade" é logado (miss é rotina, não precisa logar). Cache nunca deveria ser ponto único de falha pra uma leitura.
- Erro do `repository.GetTopSlowest` **sim** aborta (não tem mais fallback depois do Postgres).
- Repopular o cache (`cache.SetSlowest`) é **best-effort** — erro aí só loga, não impede devolver os dados já buscados.

### `adapter/worker.Collector` — coletor de `pg_stat_statements`
- **Pré-requisito de infra:** `pg_stat_statements` não vem ativo por padrão. Precisa de `shared_preload_libraries=pg_stat_statements` no `command:` do serviço `db` no compose (só funciona com o container recriado, `--force-recreate`, porque é lido na inicialização do processo) **+** `CREATE EXTENSION IF NOT EXISTS pg_stat_statements;` dentro do banco.
- **Snapshot em memória** (`map[string]statSnapshot`, chave `queryID:dbUser`) guarda os valores **cumulativos atuais** de cada leitura — não o delta. Servem só de "ponto de comparação" pra próxima rodada.
- **Sem mutex necessário** — uma goroutine só, um `time.Ticker`, processamento sequencial.
- **`pg_stat_statements` já normaliza o texto da query nativamente** (troca literais por `$1`, `$2`...) — o campo vem pronto, sem processamento extra.
- Três desfechos por linha lida: **(1) primeira vez vista** → só grava snapshot, não chama `Execute`; **(2) delta de `calls` ≤ 0** (nada novo, ou reset de estatísticas/restart do Postgres) → só atualiza snapshot; **(3) delta > 0** → calcula `avgTimeMs = timeDelta / callsDelta`, chama `analyzeUseCase.Execute`, atualiza snapshot.
- **Limitação conhecida:** snapshots vivem só em memória — todo restart da aplicação "esquece" o histórico de comparação, e a primeira leitura de cada query pós-restart não gera análise (some sem gerar delta), só recomeça o baseline.
- **Achado em produção (self-referência):** o próprio `SELECT` do coletor contra `pg_stat_statements`, e o `INSERT ... ON CONFLICT` do `Save`, aparecem como queries monitoradas — a ferramenta se autoexamina. Sem filtro, isso polui o "top mais lentas" com ruído interno. Ainda não filtrado — opções: excluir por texto da query (`LIKE '%pg_stat_statements%'`), ou por role de usuário (excluir o usuário técnico da aplicação). Baixa prioridade enquanto só um usuário (`admin`) usa o sistema.
- `Start(ctx)` roda `time.Ticker` + `select` com `ctx.Done()` — permite parar o laço de forma limpa quando o graceful shutdown for implementado.

### `adapter/worker.AlertConsumer` — consumidor de alertas (Streams)
- **Duas goroutines paralelas**, disparadas por `Start(ctx)`, sem competir entre si:
  - `runConsumeLoop` — laço contínuo (`select` com `default`, sem ticker) chamando `ConsumeNew` → `XReadGroup` com `Streams: [..., ">"]`. O `Block: 2s` do próprio Redis já dá a pausa natural, sem precisar de `time.Sleep` nem ticker.
  - `runCleanupLoop` — ticker de 1 minuto, chamando `ReclaimStale` → `XAutoClaim` com `MinIdle: 3min` — reivindica, num único comando atômico, mensagens entregues há mais de 3 minutos sem `XAck` (sinal de consumidor que travou/caiu).
- **Convergência em `processMessage`**: os dois caminhos (mensagem nova ou reclamada) terminam na mesma função — `parseMapToAlert` (reconstrói o struct a partir do `map[string]any`) → log → `XAck`. Evita duplicar parse/confirmação em dois lugares.
- **Bug real que já caímos: nome de chave do mapa não bate com o que foi gravado no `Publish`** (`values["current_time"]` em vez de `values["current_time_ms"]`) — gera `strconv.ParseFloat: parsing "<nil>"`, silencioso, sem quebrar o fluxo. Mesma categoria do `Publisher`≠`Publish` — strings de contrato entre produtor e consumidor não têm checagem do compilador; só aparece rodando de verdade.
- **`XAck` sempre precisa ter o erro checado** (não só `_ =` ou ignorado) — se falhar, a mensagem continua na lista de pendentes mesmo já processada, e o `ReclaimStale` vai reentregar ela depois, gerando alerta duplicado.
- **`ConsumerName`** identifica o processo físico dentro do grupo (aparece no `XPENDING`). Com uma única instância da aplicação, qualquer valor fixo serve. **Decisão pendente:** se escalar pra múltiplas instâncias, precisa ser único por processo (ex: hostname + PID) — do contrário o Redis não distingue os processos.
- Log `"consumer alert: fila vazia"` aparecendo repetidamente (a cada ~2s) é comportamento esperado do `runConsumeLoop`, não erro — é o `XReadGroup` retornando vazio a cada tentativa de bloqueio, enquanto nenhuma anomalia real acontece.

### `usecase.GetQueryUseCase.Execute` — busca individual
- O mais simples dos três usecases: só repassa pra `repository.GetByID`, sem transformação nenhuma. Existe mesmo assim (em vez do handler chamar o repository direto) pra manter a porta de entrada única — espaço pra crescer regra depois (cache, log de acesso) sem precisar desacoplar o handler retroativamente.
- Handler `GetQueryById` usa **path params** (`ctx.Param`, não `ctx.Query`) pros dois segmentos da identidade: `/queries/:queryID/:dbUser` — reflete que os dois juntos formam a chave, nenhum dos dois é opcional.
- `domain.ErrQueryNotFound` → `404`; qualquer outro erro → `500`; sucesso → `200` com `mapToSlowQueryResponse` (singular, sem loop).
- **Bug real que já caímos aqui:** `ctx.JSON` não interrompe a execução da função — sem `return` logo depois de cada resposta de erro, o handler continua rodando as linhas seguintes e tenta escrever múltiplas respostas na mesma requisição (o Gin não impede, só loga aviso e o comportamento fica imprevisível). Regra fixa: todo `ctx.JSON` de erro precisa de `return` na sequência, sem exceção.

### `adapter/http` — handler + DTO
- **DTO (`SlowQueryResponse`) separado da entidade `domain.Query`** — decisão consciente (Opção B, mais rigorosa) pra não vazar tags `json:` pro domínio. `mapToSlowQueryResponseList` (plural) e `mapToSlowQueryResponse` (singular) fazem a tradução — uma pra lista, outra pra registro único.
- **`LastExecutionAt` no DTO é `time.Time` puro; `LastAnomalyAt` é `*time.Time`** — espelha exatamente os tipos da entidade (o primeiro sempre preenchido, o segundo nullable).
- **`limit` como query string (`?limit=10`)**, mas **`queryID`/`dbUser` como path params (`/queries/:queryID/:dbUser`)** — a distinção que importa: query string é modificador opcional sobre uma coleção; path param é identidade obrigatória do recurso. Como a identidade real de uma `Query` é o par `(queryID, dbUser)`, os dois entram como segmentos de path, não como query string.
- Validação do `limit`: string vazia → default (10); não-numérico → `400`; `≤ 0` → `400`; acima do teto (100) → satura no teto, não erro.
- **`GetQueryById`**: usa `ctx.Param()` (não `ctx.Query()`) pra extrair os dois segmentos. Traduz `domain.ErrQueryNotFound` pra `404` via `errors.Is` — erro esperado do domínio vira status HTTP específico, não um `500` genérico.
- **Armadilha real que já caímos: `ctx.JSON` não interrompe a execução da função.** Sem `return` logo depois de cada `ctx.JSON` de erro, o código continua rodando as linhas seguintes — múltiplas respostas HTTP sendo escritas na mesma requisição, e possível nil pointer dereference ao tentar mapear uma query que na verdade é `nil` (busca falhou). Regra fixa: todo `ctx.JSON` de erro leva um `return` na linha de baixo, sem exceção.
- `make([]SlowQueryResponse, len(...))` em vez de `var` — garante `[]` no JSON de resposta em vez de `null` quando a lista vem vazia.
- Rota registrada via **method value** (`route.GET("/queries/slowest", h.GetSlowestQueries)`) — Go gera automaticamente a função "fechada" com o receiver já embutido, sem precisar de wrapper manual. Importante: `h.Metodo` (sem parênteses) entrega a *referência*, executada a cada requisição; `h.Metodo()` (com parênteses) executaria uma vez só, na hora de registrar — erro de lógica grave se confundido.
- **`GetQueryUseCase`** — usecase "passa-through" (só repassa pra `repository.GetByID`, sem lógica própria) — existe mesmo assim pra manter o handler sem acesso direto ao repository, preservando a porta de entrada única e o espaço pra crescer regra depois (cache, log de acesso, etc.).

### Lições gerais de Go que apareceram repetidas vezes nos adapters
- **Nunca descartar erro com `_`** — mesmo em casos que "raramente falham" (ex: `json.Marshal`). O linter `errcheck` (parte do `golangci-lint`) pega isso.
- **Nome de método precisa bater exatamente com a interface** — não existe "quase implementa" em Go. `Publisher` ≠ `Publish`, `GetById` ≠ `GetByID`. O compilador só reclama na hora de *usar* o struct onde a interface é esperada, não na definição do struct isolado — por isso esses erros passam despercebidos até mais tarde.
- **Siglas em nomes de campo/método são maiúsculas inteiras**: `ID`, `DB`, não `Id`, `Db`.
- **Nome de pacote é sempre minúsculo, sem camelCase** (`redisadapter`, não `redisAdapter`).
- **Mensagens de erro começam com letra minúscula** (convenção `golangci-lint`/`revive`).
- **Variável local não deve ter o mesmo nome de um pacote importado** (*shadowing*) — `time := time.Minute`, `worker := worker.NewCollector(...)`, `handler := handler.NewHandler(...)` todos compilam, mas "escondem" o pacote a partir daquela linha; problema esperando acontecer se precisar usar o pacote de novo mais adiante na mesma função. Prefira nomes curtos alternativos (`h`, `collector`, `collectInterval`).

---

## 8. Roteiro do que falta (ordem sugerida)

1. ~~`main.go` mínimo (builda)~~ ✅
2. ~~`config/postgres.go` + Ping~~ ✅
3. ~~`config/redis.go` + Ping~~ ✅
4. ~~Servidor HTTP vivo (`/health`)~~ ✅
5. ~~Entidade `Query` desenhada + `RegisterExecution` (Welford's/z-score) implementado~~ ✅
6. ~~Migration da tabela `queries`~~ ✅
7. ~~Interfaces `QueryRepository`, `AlertPublisher`, `QueryCache`~~ ✅
8. ~~`usecase.AnalyzeQueryUseCase.Execute`~~ ✅
9. ~~`adapter/postgres.PostgresQueryRepository`~~ ✅
10. ~~`adapter/redis.StreamAlertPublisher` (produtor + `EnsureConsumerGroup`)~~ ✅
11. ~~`adapter/redis.QueryCacheAdapter` (Set/Get)~~ ✅
12. ~~`usecase.ListSlowestQueriesUseCase` (cache-aside)~~ ✅
13. ~~Wire completo no `main.go`~~ ✅
14. ~~`adapter/worker.Collector` (lê `pg_stat_statements`, calcula delta, chama `Execute`)~~ ✅
15. ~~`adapter/http` — handler `GET /queries/slowest` + DTO~~ ✅ (testado e funcionando de ponta a ponta)
16. ~~Handler `GET /queries/{queryID}/{dbUser}` (path params, identidade composta)~~ ✅ (testado e funcionando)
17. ~~Filtro de auto-referência no coletor~~ ✅
18. ~~`adapter/worker.AlertConsumer` — consumidor (`XReadGroup`) + "faxineiro" de pendências (`XAutoClaim`)~~ ✅ (testado e funcionando)
19. Tabela + mapeamento usuário → contato (e-mail/Slack) — ainda em aberto conceitualmente
20. ~~Testes unitários (domain/usecase) + integração (`testcontainers-go` para Postgres)~~
21. `golangci-lint` configurado
22. `.github/workflows/ci.yml` — lint → testes → build da imagem Docker
23. Graceful shutdown (`SIGTERM`/`SIGINT` fechando pool/client/coletor/consumidor antes de encerrar)
24. README com diagrama de arquitetura + decisões documentadas (reaproveitar as justificativas deste guia)

---

## 9. Comandos do dia a dia

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

## 10. Troubleshooting — lições aprendidas

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