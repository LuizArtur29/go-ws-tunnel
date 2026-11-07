# go-ws-tunnel

Túnel SSH sobre WebSocket e TLS, desenvolvido em Go para aprender redes,
concorrência, half-close e controle de acesso.

**Estado atual:** túnel SSH implementado e validado pela porta pública 443,
com autenticação por chave, terminal interativo, SFTP, sessões simultâneas e
encaminhamento de portas. O diagnóstico `/probe` permanece disponível.
O teste na rede institucional e a interface do VS Code ainda não foram executados.

## Arquitetura

```text
SSH / SCP / SFTP / Remote SSH
          ↓ stdin/stdout (ProxyCommand) ou TCP loopback
       cliente Go
          ↓ WebSocket + TLS — porta 443
         Nginx
          ↓ TLS sobre socket Unix compartilhado
       servidor Go
          ↓ TCP loopback — destino fixo configurado no servidor
        OpenSSH
```

O OpenSSH continua autenticando o usuário por chave e verificando a identidade
do servidor. O token autoriza apenas o canal de transporte. Não implementamos
criptografia própria ou o protocolo SSH.

A porta 443 aberta não garante funcionamento em qualquer rede: WebSocket,
domínio, IP, proxy obrigatório e duração das conexões podem sofrer restrições.

## Compilar e verificar

Go 1.24 ou superior:

```bash
make build
make check
```

Os binários ficam em `bin/probe` e `bin/tunnel`. O Makefile usa `go build`,
`go vet`, `go test -race` e `gofmt`. A dependência WebSocket é
[`coder/websocket`](https://github.com/coder/websocket), fixada em `go.mod`.

## Usar SSH

Configure uma entrada SSH com `ProxyCommand` conforme o
[guia do túnel](docs/ssh-tunnel.md), usando domínio, chave e token próprios:

```bash
ssh minha-vps-wss
scp arquivo.txt minha-vps-wss:/CAMINHO/DO/USUARIO/
sftp minha-vps-wss
```

Também há cliente com porta local e exercício TCP sem WebSocket:

```bash
./bin/tunnel client -url wss://tunel.example.com/ssh \
  -token-file local/access.token -listen 127.0.0.1:2222

./bin/tunnel tcp-proxy -listen 127.0.0.1:2222 -target 127.0.0.1:22
```

O segundo comando é apenas um exercício local. Para ativar encaminhamento no
servidor TLS, use `probe server ... -ssh-target 127.0.0.1:22`. Sem essa opção,
`/ssh` fica desabilitada. O template systemd inclui essa opção explicitamente.

## Laboratório de diagnóstico

Linux/macOS com OpenSSL:

```bash
mkdir -p local
chmod 700 local
openssl rand -hex 32 > local/access.token
chmod 600 local/access.token
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout local/server.key -out local/server.pem -days 7 \
  -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost'
chmod 600 local/server.key
```

Em um terminal:

```bash
./bin/probe server -listen 127.0.0.1:8443 \
  -cert local/server.pem -key local/server.key -token-file local/access.token
```

Em outro:

```bash
./bin/probe client -url wss://localhost:8443/probe \
  -token-file local/access.token -ca local/server.pem -duration 30s
```

`-ca` adiciona confiança explícita ao certificado de laboratório. A verificação
de certificado e hostname permanece ativa. Na borda pública com certificado
válido, não passe `-ca`.

## Organização e documentação

| Caminho | Responsabilidade |
| --- | --- |
| `cmd/probe` | Servidor TLS, diagnóstico e encaminhamento opcional |
| `cmd/tunnel` | ProxyCommand, porta local e exercício TCP |
| `internal/probe` | Diagnóstico binário e autorização compartilhada |
| `internal/tunnel` | Protocolo v1, relay bidirecional e destino fixo |
| `deploy` | Templates Nginx/systemd e operação |
| `.github/workflows` | Verificação automática de Go |

- [Como estudar o código](docs/learning-path.md).
- [SSH, SCP, SFTP e configuração para VS Code](docs/ssh-tunnel.md).
- [Implantação e operação](deploy/README.md).
- [Diagnóstico e validação da rede](docs/network-validation.md).
- [Primeiro teste na rede de uso](docs/first-network-test.md).
- [Evidências executadas e limites](docs/validation-results.md).

## Etapas

- [x] Diagnóstico WSS autenticado e validação externa na porta 443.
- [x] Proxy TCP com half-close e cancelamento.
- [x] Transporte TCP por WebSocket com EOF por direção e keepalive.
- [x] Destino fixo loopback configurado no servidor.
- [x] SSH real, sessões simultâneas e SFTP no teste interno de preparação.
- [x] Templates systemd/Nginx, guias de uso e roteiro de teste.
- [x] Ativar `/ssh` na borda e validar SSH/SFTP pela conexão pública.
- [ ] Validar interface VS Code Remote SSH quando o aplicativo estiver disponível.
- [ ] Validar funcionamento na rede institucional.

## Controle de acesso e limites

O servidor limita `/ssh` a oito sessões e `/probe` a outras oito. O cliente TCP
escuta apenas em loopback. Mensagens do túnel têm até 32 KiB de dados; dados e
EOF usam frames distintos. O subprotocolo versionado evita confundir diagnóstico
e transporte. O destino não é escolhido pelo cliente.

O token é relido por handshake; sua substituição revoga novas conexões. Sessões
abertas precisam ser encerradas separadamente. O serviço detecta falhas com
ping/pong; não reconecta silenciosamente uma sessão SSH quebrada.

Certificados internos têm renovação manual. Ainda não há gestão de usuários,
revogação individual ou limitação de tentativas por IP. Esta implementação é um
projeto de aprendizado; resultados executados estão documentados separadamente.

Tokens, chaves, certificados, configuração pessoal e dados da IDE ficam fora do
Git. Os exemplos públicos usam endereços e caminhos fictícios.

## Licença

A licença ainda será escolhida pelo autor. Repositório público, por si só, não
concede uma licença geral de uso e redistribuição.
