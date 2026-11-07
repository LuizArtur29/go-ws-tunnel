# Usar SSH pelo túnel

## Arquitetura

```text
OpenSSH → stdin/stdout ou TCP loopback → cliente Go → WSS:443
                                                    ↓
OpenSSH local:22 ← servidor Go ← TLS/socket Unix ← Nginx
```

O servidor `probe` mantém `/probe` e pode habilitar `/ssh` com
`-ssh-target 127.0.0.1:22`. Sem essa flag, `/ssh` não está habilitada.
O destino é um IP loopback literal e porta definidos no servidor; o cliente
não envia nem escolhe destinos. O template systemd já habilita o SSH local.
A configuração Nginx de exemplo inclui as duas rotas.

O cliente e servidor negociam o subprotocolo `go-ws-tunnel.v1`. Cada sessão
transporta um fluxo TCP distinto, em mensagens binárias limitadas a 32 KiB de
payload. Um byte inicial identifica dados ou EOF. EOF é por direção: fechar
a entrada não interrompe a resposta. O relay mantém o leitor WebSocket ativo
para processar controles mesmo após EOF de uma direção.

Ping a cada 20 segundos, com timeout de 10 segundos, verifica conexões ociosas.
Uma operação de escrita WebSocket tem limite de 45 segundos. Não há reconexão
silenciosa: uma conexão SSH existente não pode continuar em um novo fluxo TCP.
Para processos longos, use uma sessão persistente no servidor de acordo com
seu fluxo de trabalho e reconecte o SSH se a rede cair.

A biblioteca WebSocket implementa framing, TLS é da biblioteca padrão e o
OpenSSH implementa autenticação e criptografia SSH. O projeto implementa o
transporte, o half-close, o ciclo de vida e o controle de acesso ao canal.

## Compilar

```bash
make build
make check
```

Ou execute os comandos Go equivalentes do Makefile. Não há dependência de
Python na aplicação. Tokens, configurações pessoais e certificados ficam em
`local/`, fora do Git.

## Opção 1: ProxyCommand (recomendado)

Crie uma entrada no seu arquivo SSH. Substitua todos os exemplos abaixo:

```sshconfig
Host minha-vps-wss
    HostName IP_REAL_DA_VPS
    User SEU_USUARIO
    IdentityFile ~/.ssh/SUA_CHAVE
    IdentitiesOnly yes
    HostKeyAlias IP_REAL_DA_VPS
    StrictHostKeyChecking yes
    ServerAliveInterval 15
    ServerAliveCountMax 3
    ProxyCommand /CAMINHO/ABSOLUTO/bin/tunnel stdio -url wss://tunel.example.com/ssh -token-file /CAMINHO/ABSOLUTO/local/access.token
```

O `HostKeyAlias` deve coincidir com a identidade do servidor já verificada no
seu `known_hosts` por um canal confiável. Não use `StrictHostKeyChecking=no`
nem aceite automaticamente uma identidade diferente. Nenhuma chave SSH privada
é enviada para o serviço do túnel.

```bash
ssh minha-vps-wss
ssh minha-vps-wss 'uname -s'
scp arquivo.txt minha-vps-wss:/CAMINHO/DO/USUARIO/
sftp minha-vps-wss
```

O modo `stdio` reserva stdout exclusivamente aos bytes SSH; mensagens de erro
vão para stderr. Não é preciso manter um cliente separado em outro terminal.

## Opção 2: porta TCP local

```bash
./bin/tunnel client -url wss://tunel.example.com/ssh \
  -token-file local/access.token -listen 127.0.0.1:2222
```

Em outro terminal:

```bash
ssh -p 2222 -o HostKeyAlias=IP_REAL_DA_VPS \
  -o StrictHostKeyChecking=yes -i ~/.ssh/SUA_CHAVE SEU_USUARIO@127.0.0.1
```

O cliente aceita apenas listeners em IP loopback literal. Cada conexão local
abre um novo WebSocket; não há multiplexação própria. O servidor limita a oito
sessões simultâneas, além do limite separado do diagnóstico. O listener local
aceita até 32 sessões em processamento.

Em computador compartilhado, outros usuários locais podem alcançar uma porta
loopback enquanto o cliente estiver ativo. Prefira ProxyCommand e mantenha a
proteção normal da sua chave SSH.

## Exercício: proxy TCP sem WebSocket

```bash
./bin/tunnel tcp-proxy -listen 127.0.0.1:2222 -target 127.0.0.1:22
```

Esse exercício encaminha duas conexões TCP locais com `io.Copy`, half-close e
cancelamento. Serve para estudar a etapa anterior ao transporte WebSocket.
Não cria acesso pela internet nem usa TLS.

## VS Code Remote SSH

A integração usa a mesma entrada SSH com ProxyCommand:

1. Instale a extensão oficial Remote - SSH, se ainda não estiver disponível.
2. Execute `Remote-SSH: Connect to Host...`.
3. Selecione `minha-vps-wss` e abra uma pasta do seu usuário na VPS.
4. Se usar arquivo SSH separado, configure `remote.SSH.configFile` para ele.

Se o ambiente não tiver VS Code/extensão, não confunda sucesso de SSH e port
forwarding com validação da interface gráfica. Registre isso separadamente.
Referência: [documentação oficial](https://code.visualstudio.com/docs/remote/ssh).

## TLS, proxy e operação

O cliente aceita `-ca` apenas para adicionar confiança em CA privada/laboratório.
Não existe opção para desabilitar validação. Pela borda com certificado público,
não passe `-ca`. O transporte HTTP usa as configurações de proxy do ambiente.

O serviço systemd usa usuário sem privilégios e socket Unix 0600. Ao atualizar
o binário ou reiniciar o serviço, sessões do túnel em andamento são encerradas;
faça atualizações fora de sessões importantes. O certificado interno ainda tem
renovação manual, descrita no runbook de implantação.

Ao compartilhar resultados, publique apenas ambiente genérico, versão do código,
duração e resultados. Não publique token, chaves ou configuração real da VPS.
