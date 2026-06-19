# VPN Control Plane (Multi-Cluster)

> **Versão atual:** `31ee5f4` (pré-release)

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
- `internal/domain`: entidades (`Peer`, `Cluster`), regras de negócio e contratos (`PeerRepository`, `ClusterRepository`, `VPNManager`).
- `internal/usecase`: orquestração dos casos de uso (`PeerUseCase`, `ClusterUseCase`).
- `internal/infra/sqlite`: persistência em SQLite (repositórios de peer e cluster).
- `internal/infra/wireguard`: integração com a CLI do WireGuard.
- `internal/infra/metrics`: coletor de métricas de negócio em background (total de clusters e peers).
- `internal/presentation/http`: handlers HTTP (`PeerHandler`, `ClusterHandler`).
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
| `total_peers` | GaugeVec | `cluster_id` | Total de peers por cluster |

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

## Métricas e observabilidade

A partir da versão `31ee5f4`, o projeto conta com:

- **Métricas RED** (`http_requests_total`, `http_request_duration_seconds`): coletadas via middleware HTTP para todas as requisições, permitindo monitorar taxa de erros, throughput e latência por rota.
- **Métricas de negócio** (`vpn_clusters_total`, `total_peers`): coletadas em background a cada 15 segundos pelo `CollectorService`, refletindo o estado atual do banco.
- **Endpoint `/metrics`** no formato Prometheus, pronto para ser coletado pelo Prometheus e visualizado no Grafana.

## Limitações atuais

O projeto está funcional como prova de conceito, mas ainda tem algumas limitações importantes:

- apenas endpoints de criação de cluster e peer (sem listagem, revogação ou remoção);
- não há autenticação nem autorização na API;
- parte da configuração (porta, caminho do banco) ainda está hardcoded no código;
- a integração com WireGuard depende da CLI `wg` e de permissões no host;
- o retorno de peers é texto puro, sem metadados estruturados;
- o DNS no template de configuração do cliente está fixo em `10.8.0.1`;
- métricas de negócio têm uma condição invertida no `collectMetrics` que impede a atualização correta quando o banco retorna dados (apenas atualiza em caso de erro).

## Próximos passos naturais

- adicionar endpoints para listar, revogar e remover clusters e peers;
- mover porta HTTP e caminho do banco para variáveis de ambiente;
- incluir autenticação da API;
- adicionar logs estruturados e observabilidade;
- tornar o servidor DNS configurável por cluster;
- permitir configuração de `AllowedIPs` por peer ou cluster;
- corrigir a condição do `collectMetrics` no coletor de métricas de negócio.