# go-ws-tunnel

Um túnel SSH sobre WebSocket e TLS, desenvolvido em Go para aprender redes,
concorrência e segurança, com evolução documentada em etapas.

**Estado atual: diagnóstico de conectividade.** Esta versão abre uma conexão
WebSocket autenticada sobre TLS, troca mensagens binárias e verifica sua
integridade ao longo do tempo. Ainda não encaminha SSH nem integra o VS Code.
Laboratório local e backend TLS na VPS validados. A ativação na borda HTTPS
e o teste externo ainda dependem da etapa operacional; a rede institucional
permanece fora desta validação.

## Problema e arquitetura planejada

Quando o acesso direto à porta 22 está indisponível, queremos experimentar o
transporte de SSH por uma conexão WebSocket com TLS na porta 443, em redes
onde esse uso seja autorizado.

```text
Cliente SSH → TCP local → cliente Go → WSS:443 → servidor Go → SSH local:22
```

O SSH mantém sua autenticação por chave e a verificação da identidade do servidor.
O túnel terá autenticação própria e um destino fixo configurado na VPS.
A porta 443 aberta não garante funcionamento: WebSocket, IP, domínio e conexões
longas também podem sofrer restrições. Primeiro medimos a conectividade real.

## Ferramentas

- Go 1.24 ou superior; biblioteca padrão para rede, HTTP, TLS e contextos.
- [`coder/websocket`](https://github.com/coder/websocket), versão fixada em `go.mod`,
  para implementar o protocolo WebSocket. A lógica do túnel será nossa.
- OpenSSL para gerar certificados de laboratório.
- OpenSSH como destino na etapa futura.
- `go test`, detector de corridas e `go vet` para validação.

O diagnóstico termina TLS no próprio Go. Não exige Caddy, Docker ou banco de dados.

## Compilar e testar

Na raiz do repositório:

```bash
go mod download
go test -race ./...
go vet ./...
mkdir -p bin
go build -o bin/probe ./cmd/probe
```

A integração contínua executa formatação, análise estática, testes e compilação.
Os testes locais precisam poder abrir sockets em localhost.

## Laboratório local

Os comandos abaixo são para Linux/macOS, com Go e OpenSSL instalados.

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

O cliente imprime o tempo de ida e volta de cada mensagem e termina com `OK`
se todas as trocas forem verificadas. Falhas encerram o processo com código 1.
`-ca` adiciona confiança explícita ao certificado de laboratório; não desabilita
verificação de certificado ou de hostname. Não há opção `insecure`.

## Teste na VPS e na rede de uso

Consulte [o roteiro de validação](docs/network-validation.md) antes da implantação.
Ele descreve TLS, porta 443, proxy e como registrar os resultados sem publicar
credenciais ou detalhes da rede.

Resultados executados: [registro de validação](docs/validation-results.md).
Templates operacionais: [implantação](deploy/README.md).

## Organização

```text
cmd/probe/              CLI de diagnóstico: server e client
internal/probe/         autenticação, echo binário e verificação dos dados
docs/                  arquitetura, aprendizado e validação de rede
.github/workflows/     verificações automáticas
```

## Etapas

- [x] Documentar escopo e stack.
- [x] Implementar diagnóstico WSS autenticado com testes locais de integração.
- [x] Executar laboratório local e testar falhas de autenticação, TLS e interrupção.
- [x] Instalar e testar backend TLS isolado na VPS.
- [ ] Ativar rota na borda HTTPS e validar conexão externa pela porta 443.
- [ ] Validar na rede de uso (adiado até haver acesso à rede).
- [ ] Implementar proxy TCP local para estudar fluxos e encerramento de conexões.
- [ ] Transportar o fluxo TCP por WebSocket.
- [ ] Restringir destino ao SSH local e testar sessão SSH real.
- [ ] Validar transferência de arquivos e VS Code Remote SSH.
- [ ] Adicionar operação via systemd e limites para uso contínuo.

## Segurança e limites desta etapa

O endpoint apenas devolve mensagens binárias de até 4 KiB e aceita até oito
sessões simultâneas. Exige token, rejeita `Origin` de navegador e não abre
conexões a outros destinos. O token é lido a cada handshake: sua substituição
revoga novas conexões, mas sessões abertas precisam ser encerradas separadamente.
Mensagens inativas por dois minutos são encerradas. Não implementamos ainda
limitação de tentativas por IP nem administração de usuários; esta é uma versão
de laboratório, sem alegação de prontidão para produção.

Tokens, certificados, chaves, arquivos locais e configurações da IDE estão no
`.gitignore`. Não publique logs com dados pessoais, IPs privados ou credenciais.

## Licença

A licença ainda será escolhida pelo autor. Repositório público, por si só, não
concede uma licença geral de uso e redistribuição.
