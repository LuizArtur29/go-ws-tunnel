# Validar WebSocket com TLS antes de implementar SSH

## Preparação

1. Disponibilizar uma VPS própria, domínio apontando para ela e certificado TLS
   válido para esse domínio. O diagnóstico usa arquivos PEM existentes; ainda
   não emite nem renova certificados automaticamente.
2. Verificar quem ocupa a porta 443. Não parar ou substituir serviços existentes;
   se houver um proxy, planejar a integração da rota `/probe` com ele.
3. Liberar a porta TCP 443 nas camadas de firewall aplicáveis à VPS.
4. Gerar um token aleatório e copiar somente o token ao computador cliente por
   um canal seguro. A chave privada TLS fica exclusivamente na VPS.

## Executar na VPS

Depois de compilar para o sistema e arquitetura corretos:

```bash
./bin/probe server -listen 0.0.0.0:443 \
  -cert /CAMINHO/fullchain.pem -key /CAMINHO/privkey.pem \
  -token-file /CAMINHO/access.token
```

Em Linux, vincular a porta 443 geralmente exige permissão específica. Planejaremos
um serviço com privilégio mínimo; não é necessário executar toda a aplicação como
root. O exemplo escuta IPv4. Um registro AAAA deve apontar para um caminho IPv6
realmente atendido ou ser removido da configuração de teste.

## Executar no cliente

Primeiro em uma rede de controle e depois na rede onde se pretende usar, com o
acesso permitido:

```bash
./bin/probe client -url wss://tunel.example.com/probe \
  -token-file local/access.token -duration 30s
```

O certificado público é validado pelas CAs do sistema; não passe `-ca` nesse caso.
Os exemplos usam domínio fictício. Não coloque token na URL ou no comando: o
conteúdo é lido do arquivo e enviado no header Authorization dentro de TLS.

Para testar maior duração:

```bash
./bin/probe client -url wss://tunel.example.com/probe \
  -token-file local/access.token -duration 15m
```

Para testar intervalos de ociosidade de 60 segundos:

```bash
./bin/probe client -url wss://tunel.example.com/probe \
  -token-file local/access.token -duration 5m -interval 60s
```

Use intervalos menores que dois minutos, limite de ociosidade do servidor.
O teste mantém uma única conexão e não reconecta silenciosamente. Uma última
troca verifica a conexão ao atingir a duração pedida; cada troca tem timeout de
10 segundos. O handshake inicial tem timeout de 15 segundos.

O transporte HTTP usa as configurações de proxy do ambiente (`HTTPS_PROXY`,
`HTTP_PROXY`, `NO_PROXY`) da biblioteca padrão Go. Proxies com autenticação
institucional específica podem exigir suporte adicional. Não configure credenciais
de proxy no repositório ou em registros compartilhados.

## Registrar evidências

| Ambiente | Duração pedida | Intervalo | Trocas verificadas | Resultado |
|---|---|---|---|---|
| Local com certificado de laboratório | 30s | 1s | a preencher | pendente |
| VPS pela rede de controle | 30s | 1s | a preencher | pendente |
| VPS pela rede de uso | 15m | 1s | a preencher | pendente |
| VPS pela rede de uso, com ociosidade | 5m | 60s | a preencher | pendente |

Registre também versão do commit e data. Use rótulos genéricos para ambientes.
Um sucesso comprova apenas que essa conexão funcionou naquele momento; não
garante a futura sessão SSH, desempenho sustentado ou compatibilidade com VS Code.

Falha de handshake pode indicar DNS, TLS, proxy ou restrição de rede. HTTP 401
indica autenticação rejeitada; 404, rota incorreta; 503, indisponibilidade de
credencial ou limite de sessões. Investigue sem desabilitar validação TLS.
