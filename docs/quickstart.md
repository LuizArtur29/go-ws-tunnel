# Laboratório local: SSH sobre WebSocket e TLS

Este laboratório conecta um cliente Go ao OpenSSH da própria máquina por WSS,
sem Nginx ou domínio público. Ele permite entender o transporte antes da
[implantação na VPS](../deploy/README.md).

## Requisitos

- Linux, Go 1.24 ou superior, Git, Make e OpenSSL.
- Cliente e servidor OpenSSH instalados.
- SSH local em `127.0.0.1:22` e uma chave já autorizada para seu usuário.

Confirme primeiro que `ssh SEU_USUARIO@127.0.0.1` funciona e verifique a chave
do host por um canal confiável. O laboratório não instala ou configura o sshd.

## Compilar e preparar TLS

Na raiz do repositório:

```bash
make build
mkdir -p local
chmod 700 local
openssl rand -hex 32 > local/access.token
chmod 600 local/access.token
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout local/server.key -out local/server.pem -days 7 \
  -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost'
chmod 600 local/server.key
```

Esses arquivos ficam ignorados pelo Git. O certificado é somente para o
laboratório; `-ca` adiciona confiança explícita sem desativar a validação.

## Iniciar o servidor

Em um terminal, mantenha este processo em execução:

```bash
./bin/probe server -listen 127.0.0.1:8443 \
  -cert local/server.pem -key local/server.key \
  -token-file local/access.token -ssh-target 127.0.0.1:22
```

O processo atende `/probe` e `/ssh` com TLS na porta local 8443. A porta pública
443 entra na implantação com Nginx; neste laboratório tudo fica em loopback.

## Conectar por ProxyCommand

Adicione ao seu `~/.ssh/config`, substituindo usuário, chave e caminhos absolutos:

```sshconfig
Host tunnel-lab
    HostName 127.0.0.1
    User SEU_USUARIO
    IdentityFile ~/.ssh/SUA_CHAVE
    IdentitiesOnly yes
    HostKeyAlias 127.0.0.1
    StrictHostKeyChecking yes
    ProxyCommand /CAMINHO/go-ws-tunnel/bin/tunnel stdio -url wss://localhost:8443/ssh -token-file /CAMINHO/go-ws-tunnel/local/access.token -ca /CAMINHO/go-ws-tunnel/local/server.pem
```

O `HostKeyAlias` usa a identidade já verificada na conexão SSH local.

```bash
ssh tunnel-lab 'printf "ssh pelo tunel: ok\n"'
ssh tunnel-lab
```

## Conferir uma transferência

```bash
printf 'Teste de transferencia pelo tunel\n' > /tmp/tunnel-send.txt
scp /tmp/tunnel-send.txt tunnel-lab:/tmp/tunnel-remote.txt
scp tunnel-lab:/tmp/tunnel-remote.txt /tmp/tunnel-return.txt
sha256sum /tmp/tunnel-send.txt /tmp/tunnel-return.txt
```

Os dois hashes devem ser iguais. Encerre o servidor com Ctrl+C ao terminar.
Os arquivos de teste em `/tmp` podem ser removidos depois da conferência.

Para o uso público, siga o [guia SSH](ssh-tunnel.md): configure Nginx na porta
443, certificado público e credenciais próprias. A CA deste laboratório não
deve ser usada para o domínio público.
