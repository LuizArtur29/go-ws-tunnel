# Como estudar esta primeira etapa

1. Leia `cmd/probe/main.go`: flags, sinais do sistema, certificado e contexto.
2. Leia `internal/probe/probe.go`: autorização HTTP ocorre antes do upgrade
   WebSocket. O servidor devolve bytes e o cliente verifica a resposta.
3. Compare o handshake HTTP com as mensagens que trafegam depois do upgrade.
4. Observe que TLS é configurado no servidor HTTP e no transporte do cliente.
5. Execute os testes: credencial inválida, certificado não confiável, limite de
   tamanho, texto inválido, rotação de token e cancelamento.
6. Rode o laboratório em dois terminais. Interrompa o servidor e observe o
   cliente falhar, sem fingir que houve continuidade de sessão.

## Próximo exercício: TCP

Construir um proxy local com `net.Listen`, `net.DialContext` e `io.Copy`.
Testar encerramento de cada lado, cancelamento e múltiplas sessões. Depois
substituir o trecho entre cliente e servidor pelo transporte WebSocket.

O OpenSSH continuará implementando SSH. Construiremos o túnel que transporta
seus bytes, sem criar criptografia ou autenticação SSH próprias.

## Referências

- https://pkg.go.dev/net
- https://pkg.go.dev/net/http
- https://pkg.go.dev/crypto/tls
- https://pkg.go.dev/context
- https://pkg.go.dev/github.com/coder/websocket

## O caminho de uma troca de dados

1. `signal.NotifyContext` permite interromper o processo pelo terminal.
2. O servidor HTTP recebe o handshake HTTPS em `/probe`.
3. O handler lê o token do arquivo e compara hashes em tempo constante antes
   de aceitar o upgrade WebSocket. A credencial viaja no header dentro de TLS.
4. `websocket.Accept` passa de HTTP a uma conexão com mensagens binárias.
5. O cliente cria 64 bytes aleatórios; `RoundTrip` envia, lê e compara a resposta.
6. O contexto limita cada operação. Uma falha encerra o diagnóstico; não há
   reconexão que possa esconder uma interrupção no teste.

O tempo impresso é o de ida e volta da mensagem, não apenas latência de rede.
Inclui processamento e agendamento dos dois processos. A autenticação desse
canal não substitui a autenticação SSH, que será implementada pelo OpenSSH.
