# Rascunho para o LinkedIn

Construí um túnel SSH em Go e validei o acesso à minha VPS pela rede do IF.

O ponto de partida foi um problema do meu dia a dia: nessa rede, a porta 22 era
bloqueada, o que dificultava acessar o ambiente onde desenvolvo. Usei isso como
projeto de estudo de redes e concorrência em Go.

O caminho da conexão ficou assim:
OpenSSH → cliente Go → WebSocket + TLS na porta 443 → Nginx → servidor Go → OpenSSH da VPS.

Implementei o transporte bidirecional, encerramento independente de cada direção
(half-close), keepalive, cancelamento e controle de acesso por token. Usei uma
biblioteca para WebSocket, TLS da biblioteca padrão e o próprio OpenSSH para
autenticação e criptografia SSH.

Na rede do IF, validei terminal remoto, execução de comandos, retomada da sessão
após ociosidade e transferência de arquivos com SCP. Um arquivo de 10 MiB foi
enviado e baixado, com SHA-256 idêntico na origem e no retorno.

Um dos aprendizados mais interessantes foi perceber que estabelecer a conexão
é só uma parte do trabalho: preservar EOF por direção, encerrar goroutines e
lidar com falhas sem simular continuidade de uma sessão exigiu bastante cuidado.

O repositório está público, sob licença MIT, com laboratório local, arquitetura,
testes automatizados e os resultados das validações. É um projeto de aprendizado
funcional; o resultado observado nessa rede não garante compatibilidade com toda
rede ou proxy.

Código e documentação: https://github.com/LuizArtur29/go-ws-tunnel

#Golang #Go #Redes #WebSocket #SSH #OpenSource

---

## Antes de publicar

- Faça commit e publique a documentação para que o link leve à versão revisada.
- Se incluir uma imagem, prefira o diagrama de arquitetura ou uma captura com
  domínio, IP, usuário e credenciais ocultados.
- O VS Code Remote SSH ainda não foi validado na interface gráfica; o texto
  descreve apenas os resultados registrados.
