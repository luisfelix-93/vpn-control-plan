# VPN Control Plane

API HTTP em Go para provisionar peers WireGuard de forma simples. O serviço gera um par de chaves para o cliente, escolhe o próximo IP livre da rede, aplica o peer na interface WireGuard do host, persiste o estado em SQLite e devolve o arquivo de configuração do cliente.

## Objetivo

Este projeto implementa um control plane enxuto para uma VPN WireGuard. Hoje o fluxo principal exposto pela aplicação é o cadastro de um novo peer via HTTP.

## Como funciona

Ao receber uma requisição para criar um peer, a aplicação executa este fluxo:

1. Gera um par de chaves WireGuard.
2. Cria a entidade de domínio do peer.
3. Consulta os IPs já usados no banco SQLite.
4. Calcula o próximo IP disponível da rede.
5. Aplica o peer na interface WireGuard do host com o comando `wg set`.
6. Salva o peer no banco.
7. Gera e retorna o arquivo de configuração do cliente.

## Arquitetura

O projeto está organizado em camadas:

- `cmd/api`: ponto de entrada da aplicação.
- `internal/domain`: entidades, regras de negócio e contratos.
- `internal/usecase`: orquestração do caso de uso de registro de peers.
- `internal/infra/sqlite`: persistência em SQLite.
- `internal/infra/wireguard`: integração com a CLI do WireGuard.
- `internal/presentation/http`: handler HTTP.

## Requisitos

- Go 1.24.4 ou compatível com o módulo.
- WireGuard instalado no host com o binário `wg` disponível no PATH.
- Uma interface WireGuard já existente no host, por padrão `wg0`.
- Permissão para executar comandos que alterem a configuração da interface WireGuard.
- Ambiente compatível com a CLI do WireGuard. Na prática, o adaptador atual foi desenhado para Linux.

## Configuração atual

No estado atual do código, parte da configuração está fixa em `cmd/api/main.go`:

- Banco SQLite: `./vpn.db`
- Interface WireGuard: `wg0`
- Endpoint do servidor: `vpn.meudominio.com:51820`
- Rede da VPN: `10.8.0.0/24`
- Porta HTTP: `8080`

A chave pública do servidor é lida de variável de ambiente. Importante: o código atualmente consulta `SERVER_girPUB_KEY`.

Exemplo no PowerShell:

```powershell
$env:SERVER_girPUB_KEY = "SUA_CHAVE_PUBLICA_DO_SERVIDOR"
```

Se essa variável não estiver definida, a aplicação sobe, mas o arquivo de configuração retornado ao cliente ficará incompleto.

## Executando o projeto

Instale as dependências do módulo:

```powershell
go mod download
```

Defina a chave pública do servidor:

```powershell
$env:SERVER_girPUB_KEY = "SUA_CHAVE_PUBLICA_DO_SERVIDOR"
```

Suba a API:

```powershell
go run ./cmd/api
```

Ao iniciar corretamente, o serviço:

- cria o arquivo SQLite `vpn.db` no diretório atual, se necessário;
- inicializa a tabela `peers`;
- expõe a API HTTP em `http://localhost:8080`.

## Endpoint disponível

### `POST /peers`

Registra um novo dispositivo na VPN.

Corpo da requisição:

```json
{
  "name": "iphone-do-luis"
}
```

Exemplo com `curl`:

```bash
curl -X POST http://localhost:8080/peers \
  -H "Content-Type: application/json" \
  -d '{"name":"iphone-do-luis"}'
```

Exemplo no PowerShell:

```powershell
Invoke-RestMethod \
  -Method Post \
  -Uri "http://localhost:8080/peers" \
  -ContentType "application/json" \
  -Body '{"name":"iphone-do-luis"}'
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
AllowedIPs = 10.8.0.0/24, 192.168.1.0/24
PersistentKeepalive = 25
```

Erros esperados:

- `400 Bad Request` para JSON inválido.
- `400 Bad Request` quando `name` não for informado.
- `500 Internal Server Error` para falhas na geração de chaves, aplicação do peer, persistência ou geração do arquivo de configuração.

## Banco de dados

O repositório SQLite cria automaticamente a tabela `peers`:

- `id`
- `name`
- `public_key`
- `allocated_ip`
- `is_revoked`
- `created_at`

O projeto usa SQLite local e não depende de um servidor externo de banco de dados.

## Regras de rede

O domínio trata a sub-rede informada como pool de IPs e evita:

- o endereço da rede;
- o broadcast;
- o IP final `.1`, reservado implicitamente como gateway.

Na configuração padrão `10.8.0.0/24`, o primeiro cliente recebe `10.8.0.2`.

## Testes

Para executar os testes automatizados atuais:

```powershell
go test ./...
```

Os testes hoje cobrem principalmente:

- criação e regras da entidade `Peer`;
- cálculo do próximo IP disponível na rede.

## Limitações atuais

O projeto está funcional como prova de conceito, mas ainda tem algumas limitações importantes:

- só existe o endpoint de criação de peer;
- não há autenticação nem autorização na API;
- parte da configuração ainda está hardcoded no código;
- a integração com WireGuard depende da CLI `wg` e de permissões no host;
- em caso de falha ao salvar no banco depois de aplicar o peer no host, não existe rollback automático;
- o retorno da API é texto puro, sem metadados estruturados.

## Próximos passos naturais

- mover configurações para variáveis de ambiente ou arquivo `.env`;
- adicionar endpoints para listar, revogar e remover peers;
- incluir autenticação da API;
- adicionar logs estruturados e observabilidade;
- tratar rollback e consistência entre banco e estado aplicado no WireGuard.