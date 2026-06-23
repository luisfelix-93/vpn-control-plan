# VPN Control Plane (Multi-Cluster)

> **Versão atual:** `08abfe0` (branch `feature/latency-metrics`)

API HTTP em Go para provisionar peers WireGuard com suporte a múltiplas zonas de rede (clusters). O serviço permite cadastrar clusters com configurações independentes (CIDR, interface, endpoint) e, dentro de cada cluster, provisionar peers com geração de chaves, IPAM dinâmico, aplicação na interface WireGuard e retorno do arquivo de configuração do cliente.

## Objetivo

Este projeto implementa um control plane enxuto para uma VPN WireGuard com suporte a múltiplas interfaces de rede isoladas (multi-cluster). Os fluxos expostos são: cadastro de clusters e registro de peers vinculados a um cluster.

## Como funciona

### Criação de Cluster

Antes de registrar peers, é necessário criar um cluster (zona de rede) com:

- Nome identificador
- CIDR da rede (ex: `10.8.0.0/24`)
- Nome da interface WireGuard no host (ex: `wg0`)
- Chave pública do servidor
- Endpoint público do servidor (ex: `vpn.meudominio.com:51820`)

### Registro de Peer

Ao receber uma requisição para criar um peer em um cluster existente:

1. Valida se o cluster informado existe.
2. Gera um par de chaves WireGuard para o cliente.
3. Cria a entidade de domínio do peer vinculada ao cluster.
4. Consulta os IPs já usados **naquele cluster** no banco SQLite.
5. Calcula o próximo IP disponível na sub-rede do cluster.
6. Aplica o peer na interface WireGuard do host com o comando `wg set`.
7. Salva o peer no banco (com rollback no WireGuard em caso de falha).
8. Gera e retorna o arquivo de configuração do cliente.

## Arquitetura

O projeto está organizado em camadas:

- `cmd/api`: ponto de entrada da aplicação.
- `internal/domain`: entidades (`Peer`, `Cluster`, `ClusterLatency`), regras de negócio e contratos (`PeerRepository`, `ClusterRepository`, `VPNManager`).
- `internal/usecase`: orquestração dos casos de uso (`PeerUseCase`, `ClusterUseCase`).
- `internal/infra/sqlite`: persistência em SQLite (repositórios de peer e cluster).
- `internal/infra/wireguard`: integração com a CLI do WireGuard.
- `internal/infra/metrics`: coletor de métricas de negócio em background (total de clusters e peers).
- `internal/infra/health`: serviço de health check que pinga peers periodicamente em background.
- `internal/infra/network`: utilitário de ping via syscall (`ping -c 1 -W 1`).
- `internal/presentation/http`: handlers HTTP (`PeerHandler`, `ClusterHandler`, `LatencyHandler`).
- `internal/presentation/http/middleware`: middleware de métricas RED (taxa, erros e duração das requisições).

## Requisitos

- Go 1.24.4 ou compatível com o módulo.
- Dependência Prometheus (`github.com/prometheus/client_golang`) para exposição de métricas.
- WireGuard instalado no host com o binário `wg` disponível no PATH.
- Interfaces WireGuard já existentes no host para cada cluster criado.
- Permissão para executar comandos que alterem a configuração das interfaces WireGuard.
- Ambiente compatível com a CLI do WireGuard. Na prática, o adaptador atual foi desenhado para Linux.

## Configuração

A configuração agora é dinâmica e gerenciada via API, sem valores hardcoded:

- **Banco SQLite:** `./vpn.db` (fixo em `cmd/api/main.go`)
- **Porta HTTP:** `8080` (fixa em `cmd/api/main.go`)

Os parâmetros de rede (CIDR, interface, chave pública do servidor, endpoint) são informados no momento da criação de cada cluster via `POST /clusters`.

## Executando o projeto

Instale as dependências do módulo:

```powershell
go mod download
```

Suba a API:

```powershell
go run ./cmd/api
```

Ao iniciar corretamente, o serviço:

- cria o arquivo SQLite `vpn.db` no diretório atual, se necessário;
- inicializa as tabelas `clusters` e `peers`;
- inicia o coletor de métricas em background (atualização a cada 15 segundos);
- expõe a API HTTP em `http://localhost:8080`.

## Endpoints disponíveis

### `POST /clusters`

Registra uma nova zona de rede (cluster).

Corpo da requisição:

```json
{
  "name": "vpn-principal",
  "cidr": "10.8.0.0/24",
  "interface_name": "wg0",
  "server_pub_key": "SE8Zn7RAsl5x...",
  "server_endpoint": "vpn.meudominio.com:51820"
}
```

Exemplo com `curl`:

```bash
curl -X POST http://localhost:8080/clusters \
  -H "Content-Type: application/json" \
  -d '{
    "name": "vpn-principal",
    "cidr": "10.8.0.0/24",
    "interface_name": "wg0",
    "server_pub_key": "SE8Zn7RAsl5x...",
    "server_endpoint": "vpn.meudominio.com:51820"
  }'
```

Resposta de sucesso:

- Status: `201 Created`
- Content-Type: `application/json`
- Corpo: objeto JSON com os dados do cluster criado, incluindo o `id` gerado.

Erros esperados:

- `400 Bad Request` para JSON inválido ou campos obrigatórios ausentes.
- `500 Internal Server Error` para falhas de persistência.

---

### `POST /peers`

Registra um novo dispositivo em um cluster existente.

Corpo da requisição:

```json
{
  "clusterId": "uuid-do-cluster",
  "name": "iphone-do-luis"
}
```

Exemplo com `curl`:

```bash
curl -X POST http://localhost:8080/peers \
  -H "Content-Type: application/json" \
  -d '{"clusterId": "uuid-do-cluster", "name": "iphone-do-luis"}'
```

Exemplo no PowerShell:

```powershell
Invoke-RestMethod \
  -Method Post \
  -Uri "http://localhost:8080/peers" \
  -ContentType "application/json" \
  -Body '{"clusterId":"uuid-do-cluster","name":"iphone-do-luis"}'
```

Resposta de sucesso:

- Status: `201 Created`
- Content-Type: `text/plain`
- Corpo: conteúdo do arquivo de configuração WireGuard do cliente

Exemplo de resposta:

```ini
[Interface]
PrivateKey = <chave-privada-do-cliente>
Address = 10.8.0.2/32
DNS = 10.8.0.1

[Peer]
PublicKey = <chave-publica-do-servidor>
Endpoint = vpn.meudominio.com:51820
AllowedIPs = <sub-rede-do-cluster>
PersistentKeepalive = 25
```

Erros esperados:

- `400 Bad Request` para JSON inválido, `name` ou `clusterId` ausentes.
- `500 Internal Server Error` para falhas na geração de chaves, validação do cluster, aplicação do peer, persistência ou geração do arquivo de configuração.

---

### `POST /clusters/{id}/heartbeat`

Registra um heartbeat para o cluster, atualizando seu status para `online` e o timestamp `last_heartbeat`.

Exemplo com `curl`:

```bash
curl -X POST http://localhost:8080/clusters/<id>/heartbeat
```

Resposta de sucesso:

- Status: `200 OK`
- Corpo: `ACK`

Erros esperados:

- `400 Bad Request` se o ID do cluster não for informado na URL.
- `500 Internal Server Error` se o cluster não existir ou houver falha de persistência.

---

### `POST /clusters/{id}/latencies`

Registra medições de latência entre o cluster de origem (`{id}`) e um ou mais clusters de destino. Útil para construir uma matriz de latência entre múltiplos nós da rede.

Corpo da requisição (array de objetos):

```json
[
  {
    "targetId": "uuid-do-cluster-destino",
    "latencyMs": 12.5
  }
]
```

Exemplo com `curl`:

```bash
curl -X POST http://localhost:8080/clusters/<id>/latencies \
  -H "Content-Type: application/json" \
  -d '[{"targetId": "uuid-do-cluster-destino", "latencyMs": 12.5}]'
```

Resposta de sucesso:

- Status: `200 OK`
- Corpo: mensagem de confirmação

Erros esperados:

- `400 Bad Request` se o ID do cluster de origem não for informado na URL ou o JSON for inválido.
- `500 Internal Server Error` se o cluster de origem não existir ou houver falha de persistência.

Observações:

- Latências para o próprio cluster (`sourceID == targetID`) são ignoradas silenciosamente.
- Latências com `sourceID` ou `targetID` vazios são ignoradas.

---

### `GET /metrics`

Expõe métricas no formato Prometheus para monitoramento.

Exemplo com `curl`:

```bash
curl http://localhost:8080/metrics
```

**Métricas RED** (coletadas por request):

| Métrica | Tipo | Labels | Descrição |
|---------|------|--------|-----------|
| `http_requests_total` | Counter | `method`, `route`, `status` | Total de requisições HTTP |
| `http_request_duration_seconds` | Histogram | `method`, `route` | Duração das requisições |

**Métricas de negócio** (atualizadas em background a cada 15s):

| Métrica | Tipo | Labels | Descrição |
|---------|------|--------|-----------|
| `vpn_clusters_total` | Gauge | — | Número total de clusters |
| `total_peers` | GaugeVec | `cluster_id`, `status` | Total de peers por cluster e por status |

## Banco de dados

O repositório SQLite cria automaticamente as seguintes tabelas:

### `clusters`

| Coluna          | Tipo     |
|-----------------|----------|
| `id`            | TEXT PK  |
| `name`          | TEXT     |
| `cidr`          | TEXT     |
| `interface_name`| TEXT     |
| `server_pub_key`| TEXT     |
| `server_endpoint`| TEXT    |
| `created_at`    | DATETIME |

### `peers`

| Coluna         | Tipo     |
|----------------|----------|
| `id`           | TEXT PK  |
| `cluster_id`   | TEXT FK  |
| `name`         | TEXT     |
| `public_key`   | TEXT UNIQUE |
| `allocated_ip` | TEXT UNIQUE |
| `is_revoked`   | INTEGER  |
| `created_at`   | DATETIME |

O projeto usa SQLite local e não depende de um servidor externo de banco de dados.

### `clusters` (colunas adicionais na branch `feature/health-check`)

| Coluna          | Tipo     |
|-----------------|----------|
| `status`        | TEXT     |
| `last_heartbeat`| DATETIME |

### `peers` (colunas adicionais na branch `feature/health-check`)

| Coluna      | Tipo     |
|-------------|----------|
| `status`    | TEXT     |
| `last_seen` | DATETIME |

### `cluster_latencies` (nova tabela na branch `feature/latency-metrics`)

| Coluna       | Tipo     |
|--------------|----------|
| `source_id`  | TEXT     |
| `target_id`  | TEXT     |
| `latency_ms` | REAL     |
| `measured_at`| DATETIME |

A tabela armazena medições de latência entre pares de clusters, permitindo construir uma matriz de latência para monitoramento de desempenho da rede.

## Regras de rede

O domínio trata a sub-rede de cada cluster como pool de IPs e evita:

- o endereço da rede (`.0`);
- o broadcast (`.255`);
- o IP final `.1`, reservado implicitamente como gateway.

O cálculo do próximo IP disponível é feito por cluster, isolando os pools entre si.

## Testes

Para executar os testes automatizados atuais:

```powershell
go test ./...
```

Os testes hoje cobrem principalmente:

- criação e regras da entidade `Peer`;
- cálculo do próximo IP disponível na rede.

## Health Check e Heartbeat (feature/health-check)

A partir dos commits mais recentes na branch `feature/health-check`, o projeto conta com dois mecanismos de monitoramento de conectividade:

### Heartbeat de Clusters (`POST /clusters/{id}/heartbeat`)

Mecanismo passivo: o próprio cluster (ou um agente externo) envia um POST periódico para o endpoint informando que está ativo. O servidor registra o status `online` e o timestamp `last_heartbeat`. Se um cluster deixa de enviar heartbeats, o status pode ser interpretado como `offline` por um sistema externo.

- **Endpoint:** `POST /clusters/{id}/heartbeat`
- **Resposta:** `200 OK` com body `ACK`
- **Uso planejado:** agentes rodando nos servidores WireGuard enviam heartbeats a cada N segundos/minutos.

### Health Check de Peers (ping em background)

Mecanismo ativo: um serviço background (`CheckerService`) executa periodicamente (a cada 1 minuto) um ping ICMP para cada peer não revogado, utilizando o utilitário de sistema `ping -c 1 -W 1`.

- Os peers são avaliados concorrentemente (uma goroutine por peer, sincronizadas via `sync.WaitGroup`).
- Se o ping responde, o status é atualizado para `online` e o campo `last_seen` recebe o timestamp atual.
- Se o ping falha, o status é atualizado para `offline` (o `last_seen` não é alterado).
- Para evitar escrita desnecessária no banco, a atualização só ocorre se houve mudança de status ou se o peer ficou online.
- Peers revogados ou sem IP alocado são ignorados.
- Ao final de cada ciclo, as métricas Prometheus são sincronizadas via `SyncPeerHealthMetrics`.
- A duração total do ciclo é registrada na métrica `vpn_healthcheck_cycle_duration_seconds`.

### Status Tracking

Tanto `Cluster` quanto `Peer` agora possuem campos de status:

| Entidade | Campo Status | Valores | Campo Temporal |
|----------|-------------|---------|----------------|
| Cluster  | `Status`    | `online`, `offline`, `unknown` | `LastHeartbeat` |
| Peer     | `Status`    | `online`, `offline`, `unknown` | `LastSeen` |

O valor padrão na criação é `unknown`. O status é persistido no banco SQLite e exposto nas métricas Prometheus.

### Latência entre Clusters (`POST /clusters/{id}/latencies`)

Mecanismo passivo: um agente externo (ex.: script rodando em cada nó WireGuard) mede a latência para outros clusters e envia relatórios periódicos para a API.

- O payload é um array JSON de `{targetId, latencyMs}`.
- O use case `ProcessLatencyReport` valida que o cluster de origem existe antes de registrar.
- Medições para o próprio cluster (`sourceId == targetId`) ou com IDs vazios são ignoradas.
- Os dados são armazenados na tabela `cluster_latencies` e expostos na métrica `vpn_cluster_latency_ms`.
- O `CollectorService` faz um full scan da tabela a cada 15s para atualizar a matriz completa de latência no Prometheus.

## Métricas e observabilidade

A partir da branch `feature/latency-metrics`, o projeto conta com um conjunto abrangente de métricas Prometheus, organizadas em três categorias:

### Métricas RED (coletadas por requisição)

Coletadas via middleware HTTP para todas as rotas registradas.

| Métrica | Tipo | Labels | Descrição |
|---------|------|--------|-----------|
| `http_requests_total` | Counter | `method`, `route`, `status` | Total de requisições HTTP por método, rota e status |
| `http_request_duration_seconds` | Histogram | `method`, `route` | Duração das requisições HTTP em buckets (`DefBuckets`) |

### Métricas de Health Check (atualizadas a cada ciclo de ping)

Populadas pelo `CheckerService` ao final de cada ciclo de verificação de peers.

| Métrica | Tipo | Labels | Descrição |
|---------|------|--------|-----------|
| `vpn_healthcheck_cycle_duration_seconds` | Histogram | — | Duração de cada ciclo completo do health checker |
| `vpn_peers_status_total` | GaugeVec | `cluster_id`, `status` | Total de peers por cluster e status de saúde (`online`, `offline`, `unknown`) |
| `vpn_peer_last_seen_unix` | GaugeVec | `cluster_id`, `peer_id` | Timestamp Unix do último sinal de vida de cada peer |

### Métricas de Heartbeat de Clusters (atualizadas a cada ciclo de coleta)

Populadas pelo `CollectorService` a cada 15 segundos com base no estado atual do banco.

| Métrica | Tipo | Labels | Descrição |
|---------|------|--------|-----------|
| `vpn_clusters_total` | Gauge | — | Número total de clusters registrados |
| `total_peers` | GaugeVec | `cluster_id`, `status` | Total de peers por cluster e por status |
| `vpn_cluster_status` | GaugeVec | `cluster_id`, `status` | Indica se o cluster está no status informado (1=ativo para o status corrente, 0 para os demais) |
| `vpn_cluster_last_heartbeat_unix` | GaugeVec | `cluster_id` | Timestamp Unix do último heartbeat recebido |
| `vpn_cluster_heartbeat_age_seconds` | GaugeVec | `cluster_id` | Idade em segundos do último heartbeat |
| `vpn_cluster_heartbeats_total` | CounterVec | `cluster_id`, `result` | Total acumulado de heartbeats recebidos por cluster e resultado (`success`, `error`) |

### Métricas de Latência (atualizadas a cada ciclo de coleta)

| Métrica | Tipo | Labels | Descrição |
|---------|------|--------|-----------|
| `vpn_cluster_latency_ms` | GaugeVec | `source_id`, `target_id` | Latência em milissegundos reportada entre dois nós da rede |

### Endpoint

Todas as métricas são expostas em `GET /metrics` no formato Prometheus, prontas para coleta pelo Prometheus e visualização no Grafana.

## Limitações atuais

O projeto está funcional como prova de conceito, mas ainda tem algumas limitações importantes:

- não há autenticação nem autorização na API;
- parte da configuração (porta, caminho do banco) ainda está hardcoded no código;
- a integração com WireGuard depende da CLI `wg` e de permissões no host;
- o retorno de peers é texto puro, sem metadados estruturados;
- o DNS no template de configuração do cliente está fixo em `10.8.0.1`;
- o health check de peers depende do utilitário `ping` do sistema operacional (Linux);
- heartbeats de cluster são puramente informativos — não há lógica de timeout automático para marcar clusters como `offline`;
- latências entre clusters são puramente informativas — não há alertas automáticos baseados em latência alta;
- o `collectLatency` duplica a lógica de atualização de `TotalClusters` e `TotalPeers` que já existe no `collectMetrics` — pode causar inconsistência se a ordem de chamada não for controlada;
- a métrica `total_peers` é atualizada tanto pelo `collectMetrics` (via `SyncPeerHealthMetrics`) quanto pelo `collectLatency` (com label `StatusUnknown`) — há risco de sobrescrita concorrente.

## Próximos passos naturais

- adicionar endpoints para listar, revogar e remover clusters e peers;
- mover porta HTTP e caminho do banco para variáveis de ambiente;
- incluir autenticação da API;
- adicionar logs estruturados e observabilidade;
- tornar o servidor DNS configurável por cluster;
- permitir configuração de `AllowedIPs` por peer ou cluster;
- implementar timeout automático para heartbeat (ex.: marcar cluster como `offline` se não receber heartbeat por N minutos);
- migrar a coleta de latência para usar apenas `collectLatency` e remover duplicação com `collectMetrics`;
- consolidar a atualização de `total_peers` em um único ponto para evitar sobrescrita concorrente;
- adicionar testes automatizados para `CheckerService`, `CollectorService`, `ProcessLatencyReport` e handlers.