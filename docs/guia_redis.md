# Guia de estudo — `redis.Client` (go-redis v9)

Material de consulta sobre a lib `github.com/redis/go-redis/v9`, baseado no que já foi usado no projeto SQL Analyzer + o que existe mas ainda não foi tocado. Não é tutorial do zero — assume que você já sabe o que é Redis, é uma referência rápida pra tirar dúvida sem precisar perguntar de novo.

---

## 1. O que é o `redis.Client`, mentalmente

É o objeto que representa a conexão com o servidor Redis — mas "conexão" aqui é uma simplificação. Por baixo, `redis.Client` gerencia um **pool de conexões TCP**, abrindo e reaproveitando automaticamente, parecido com o `pgxpool.Pool` do Postgres. Você não abre/fecha conexão manualmente a cada comando — cria o client uma vez (na subida da aplicação), reaproveita ele pra todos os comandos, e fecha só no encerramento (`Close()`).

**Como se cria:** `redis.NewClient(&redis.Options{...})` — não conecta de verdade nesse momento (é *lazy*, mesma lógica do `pgxpool.New`). A primeira operação real (ou um `Ping` explícito) é que testa a conexão de fato.

---

## 2. O padrão "envelope" — por que nada devolve `error` direto

Toda chamada de comando (`Get`, `Set`, `XAdd`, `Ping`, etc.) devolve um **tipo de comando** (`*StringCmd`, `*StatusCmd`, `*IntCmd`, `*XAddCmd`...), nunca o valor puro nem o erro puro direto. Pensa nisso como um envelope: dentro dele pode ter o resultado de sucesso, ou um erro — os dois cabem no mesmo "pacote", e você escolhe como abrir.

**Duas formas de abrir o envelope:**
- **`.Err() error`** — só quero saber se deu certo ou não, não preciso do valor (uso típico: `Set`, `Ping`, `XAdd` quando não preciso do ID gerado).
- **`.Result() (T, error)`** — quero o valor **e** o erro juntos, num só passo (uso típico: `Get`, quando preciso da string retornada).

```go
// Só erro:
err := client.Set(ctx, key, value, ttl).Err()

// Valor + erro:
value, err := client.Get(ctx, key).Result()
```

Por que a lib fez assim (em vez de devolver `(T, error)` direto, como é comum em Go): permite **encadear** chamadas (pipelines, transações) sem precisar checar erro imediatamente a cada passo — você monta uma sequência de comandos e só verifica o resultado no final. Pro seu uso do dia a dia, isso raramente importa; só saiba que é por isso que a API parece "diferente do Go idiomático" à primeira vista.

---

## 3. Todo comando exige `ctx context.Context` como primeiro argumento

Diferente da lib antiga (`go-redis/redis`, sem `/v9`), a versão atual propaga contexto em tudo — permite cancelamento e timeout centralizados (se a requisição HTTP que originou a chamada for cancelada, o comando Redis é abortado também, em vez de ficar pendurado). Sempre passe o `ctx` que já está fluindo pela sua cadeia de chamadas, nunca crie um `context.Background()` novo dentro de um método que já recebe `ctx` — perderia esse benefício.

---

## 4. Comandos que você já usa no projeto

### Conexão / saúde
| Comando | Uso |
|---|---|
| `Ping(ctx)` | Testa se o Redis está acessível. `.Err()` no resultado. |
| `Close()` | Encerra o pool de conexões — chamar no graceful shutdown, não a cada operação. |

### Cache simples (chave → valor)
| Comando | Uso |
|---|---|
| `Set(ctx, key, value, ttl)` | Grava com expiração embutida. `ttl = 0` significa "sem expiração". |
| `Get(ctx, key)` | Busca. Se a chave não existir, o erro é o sentinela `redis.Nil` — **não** é "Redis quebrou", é "não achei". Sempre `.Result()`, nunca só `.Err()`, porque você precisa do valor. |
| `Del(ctx, keys...)` | Remove uma ou mais chaves manualmente. |
| `Exists(ctx, keys...)` | Checa existência sem trazer o valor — mais barato que `Get` quando só quer saber "tem ou não tem". |

### Streams (mensageria)
| Comando | Uso |
|---|---|
| `XAdd(ctx, &redis.XAddArgs{Stream, Values})` | Publica uma mensagem. `Values` aceita `map[string]any`. |
| `XGroupCreateMkStream(ctx, stream, group, start)` | Cria consumer group **e** o stream, se ainda não existir. `start = "$"` significa "só mensagens novas daqui pra frente". Erro `BUSYGROUP` (checar via `strings.Contains(err.Error(), "BUSYGROUP")`) significa "grupo já existe", tratar como sucesso. |
| `XReadGroup(ctx, &redis.XReadGroupArgs{...})` | Lê mensagens novas do grupo. |
| `XAck(ctx, stream, group, ids...)` | Confirma processamento de uma mensagem. |
| `XPending(ctx, stream, group)` | Lista mensagens entregues mas não confirmadas. |
| `XClaim(ctx, &redis.XClaimArgs{...})` | Reatribui mensagem pendente de um consumidor pra outro. |

---

## 5. Erros sentinela que a lib expõe — não são "Redis quebrou"

- **`redis.Nil`** — chave/campo não encontrado. Rotina, acontece toda hora, não é falha.
- **Texto `"BUSYGROUP"` dentro do erro** — grupo já existe. A lib não expõe isso como um tipo específico (`var ErrBusyGroup`), só como texto dentro da mensagem — por isso o `strings.Contains` manual, não tem alternativa mais elegante.

Regra geral: sempre traduza esses casos pra um erro sentinela **seu**, do domínio (`domain.ErrCacheMiss`, por exemplo) — assim o resto do código nunca precisa saber que "por baixo" é Redis.

---

## 6. Pegadinhas já vividas no projeto (não repetir)

- **`Addr` é `"host:porta"` já concatenado** — diferente do pgx, que recebe host/porta como campos separados dentro de uma DSN. Você monta essa string manualmente com `fmt.Sprintf`.
- **Cuidado com a versão da lib** — `github.com/go-redis/redis` (v6/v7/v8, sem context) é diferente de `github.com/redis/go-redis/v9` (atual, com context em tudo). Se aparecerem os dois no `go.mod` ao mesmo tempo, rode `go mod tidy` depois de corrigir os imports.
- **`time.Time` não serializa sozinho** — pra guardar timestamp num `Values` de Stream (ou em qualquer valor Redis), converte pra string antes (`time.RFC3339`), nunca passa o `time.Time` cru.
- **`DB` no `redis.Options`** é o índice do banco lógico (0 a 15 por padrão) — não confundir com "nome do banco" como no Postgres. Pra esse projeto, sempre `0`.

---

## 7. Comandos que existem, mas o projeto não usa (referência pra quando precisar)

| Categoria | Comandos principais | Quando usar |
|---|---|---|
| **Hashes** | `HSet`, `HGet`, `HGetAll` | Guardar um objeto com múltiplos campos numa única chave, sem serializar tudo pra JSON — útil se um dia quiser atualizar só *um* campo de um objeto cacheado sem reescrever o resto. |
| **Listas** | `LPush`, `RPush`, `LRange` | Fila simples ordenada, sem os recursos de consumer group dos Streams — mais simples, mas sem ACK/replay. |
| **Sets** | `SAdd`, `SMembers`, `SIsMember` | Conjunto sem ordem, sem repetição — útil pra "já vi esse item?" em O(1). |
| **Sorted Sets** | `ZAdd`, `ZRange`, `ZRevRange` | Ranking ordenado por score — se um dia quiser um "ranking de queries mais problemáticas" sem reordenar toda hora, `ZADD query_id score` + `ZREVRANGE` resolveria isso nativamente. |
| **Pub/Sub** | `Subscribe`, `Publish` | Mensageria sem persistência — já descartado conscientemente pro projeto (perde mensagem se ninguém estiver ouvindo; Streams foi a escolha certa). |
| **Transações** | `TxPipeline`, `Watch` | Garantir que múltiplos comandos rodem atomicamente. Não necessário hoje (cache simples + Streams com ACK já cobrem o caso de uso). |
| **Scripting** | `Eval`, `EvalSha` | Rodar script Lua direto no servidor Redis, atômico. Avançado, fora do escopo de um projeto júnior/pleno de portfólio. |

---

## 8. Perguntas que valem sua própria investigação (não estão respondidas aqui de propósito)

- Como funciona `XAutoClaim` (versão mais moderna do `XClaim`, que reivindica em lote em vez de mensagem por mensagem)?
- O que muda usando `redis.ClusterClient` em vez de `redis.Client`, se um dia o projeto crescer pra Redis em cluster?
- Como funciona pipelining de verdade (`client.Pipeline()`) pra agrupar múltiplos comandos numa única viagem de rede?

---

*Guia complementar ao `guia_projeto_sql_analyzer.md` — mantenha os dois na mesma pasta `docs/` do repositório.*