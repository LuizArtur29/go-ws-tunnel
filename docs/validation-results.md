# Registro de validação

Registro iniciado em 7 de novembro de 2026; atualizado em 9 de novembro de 2026
(America/Recife). As medições abaixo pertencem às respectivas etapas.
Etapa: diagnóstico e túnel SSH autenticados sobre WebSocket/TLS, com validação
externa pela porta 443 e teste concluído na rede institucional do IF.

## Verificações executadas

| Verificação | Evidência | Resultado |
| --- | --- | --- |
| Compilação Go | CLI local e binário Linux amd64 sem CGO | Passou |
| Análise estática | `go vet ./...` | Passou |
| Integração e concorrência | `go test -race ./...` | Passou |
| CLI local com TLS | 31 trocas em 30,002 s na mesma conexão | Passou |
| Token inválido | Cliente encerrou com código 1 e HTTP 401 | Passou |
| Certificado não confiável | Cliente recusou conexão, código 1 | Passou |
| Servidor interrompido | Cliente detectou falha durante a sessão, código 1 | Passou |
| Backend TLS na VPS | 4 trocas em 3,002 s na interface interna | Passou |
| Saúde da aplicação existente | API e rota de infraestrutura disponíveis antes da integração | Passou |
| Borda pública 443 | Cliente externo: 27 trocas verificadas em 30,382 s | Passou |
| Ociosidade pública | 3 trocas em 65.432 s, com intervalo de 60 s | Passou |
| Autenticação pública | Token inválido rejeitado com HTTP 401 e código 1 | Passou |
| Persistência do serviço | Serviço ativo e linger habilitado após ativação | Passou |
| Saúde após ativação | API UP e rota de infraestrutura respondendo normalmente | Passou |
| Rede do IF: sessão SSH | Alias `pagefy-wss` conectou à VPS; comandos remotos executados | Passou |
| Rede do IF: inspeção Docker | `sudo docker compose ps` e consulta do projeto edge | Passou |
| Rede do IF: SCP pequeno | Envio e retorno de arquivo de 50 bytes; SHA-256 igual | Passou |
| Rede do IF: SCP 10 MiB | Upload em 11 s (~923 KB/s), download em 43 s (~235 KB/s); SHA-256 igual | Passou |
| Rede do IF: sessão ociosa | Sessão mantida ociosa e retomada com comando | Passou |

Os testes automatizados também cobrem rota incorreta, query inesperada, Origin
de navegador, header de autenticação duplicado, rotação do token, mensagem de
texto, tamanho excessivo, limite de sessões e cancelamento do servidor.

## Limites da evidência

O laboratório usou certificado de teste explicitamente confiado com `-ca`; o
cliente continuou verificando identidade e cadeia. O backend da VPS usa outro
certificado interno. A borda deve continuar usando seu certificado público.
Nenhuma chave privada, token, endereço interno real ou log bruto integra este
registro público. Os processos locais de teste foram encerrados ao terminar.

Os testes locais e internos da VPS, isoladamente, não comprovavam passagem pela
rede institucional. A validação do IF foi feita separadamente e está registrada
abaixo.

## Correção da integração com a borda

A primeira ativação passou em `nginx -t`, mas a conexão pública retornou
HTTP 504. O script restaurou a configuração anterior e a API permaneceu UP.
O backend Go estava ativo. A evidência indica falha no caminho TCP entre
container e host; a regra específica de rede responsável não foi confirmada.

O backend passou a oferecer TLS sobre socket Unix compartilhado. Isso elimina
a dependência desse trecho em TCP/bridge sem alterar regras de firewall.
Testes com `-race` cobrem round-trip WSS/TLS pelo socket, modo 0600, preservação
de arquivos existentes e remoção do socket no fechamento normal.
O operador executou a nova ativação com sucesso: configuração Nginx válida,
quatro trocas em 3,003 s pela borda e saúde da aplicação preservada. Em seguida,
o cliente externo confirmou o caminho computador → HTTPS:443 → Nginx → TLS
sobre socket Unix → Go. O certificado público foi validado pelas CAs do sistema,
sem `-ca` nem desativação de verificações.

O teste externo de 30 segundos verificou 27 respostas na mesma conexão, com
RTT observado entre aproximadamente 185 e 193 ms. O teste de ociosidade
verificou novamente a conexão após 60 segundos sem mensagens. Isoladamente,
esses testes de diagnóstico não comprovam transporte SSH. A validação SSH
foi realizada depois e está registrada a seguir. Naquela etapa, a rede institucional e a
interface do VS Code ainda não tinham sido validadas.

## Implementação do túnel SSH

Implementados `tunnel stdio`, `tunnel client` e `tunnel tcp-proxy`, além da
rota `/ssh` opcional no servidor e subprotocolo `go-ws-tunnel.v1`.

| Teste | Resultado |
| --- | --- |
| Relay WSS binário de 4 MiB com half-close | Passou |
| EOF reverso, mantendo a outra direção gravável | Passou |
| Quatro sessões concorrentes de relay | Passou |
| Cancelamento e fechamento dos workers | Passou |
| Auth, subprotocolo, limite e frames inválidos | Passou |
| CLI TCP, stdio e loopback com 2 MiB | Passou |
| CLI detecta backend encerrado e token inválido | Passou |
| SSH real com chave e identidade verificadas | Passou também pela borda pública 443 |
| SSH stdin/stdout binário de 1 MiB | Passou também pela borda pública 443 |
| Quatro sessões SSH reais simultâneas | Passou também pela borda pública 443 |
| Upload/download SFTP de 1 MiB com SHA-256 | Passou também pela borda pública 443 |
| Terminal interativo com PTY | Passou também pela borda pública 443 |
| Encaminhamento SSH local de portas | Passou também pela borda pública 443 |
| Encaminhamento dinâmico SOCKS5 | Passou também pela borda pública 443 |
| Sessão SSH de 65 segundos | Passou também pela borda pública 443 |
| SSH e SFTP pela borda pública 443 | Passou |
| Modo cliente TCP local com sessão SSH real | Passou pela borda pública 443 |
| Token inválido no endpoint SSH público | Rejeitado com HTTP 401 |
| Interface gráfica VS Code Remote SSH | Não executado: aplicativo indisponível |
| Rede institucional | Concluído posteriormente; veja a seção de validação no IF |

O teste interno usou um encaminhamento SSH administrativo temporário até o
socket TLS para verificar a aplicação antes de liberar `/ssh` no Nginx.
Ele não comprova sozinho que a rota pública de SSH está ativa. O encaminhamento
de preparação e os arquivos temporários remotos foram removidos ao finalizar.
Os testes automatizados de concorrência e encerramento passaram também em
dez execuções consecutivas com o detector de corridas.

O cliente foi compilado para uso local e a entrada SSH privada foi preparada.
O CI do commit de implementação passou. O ambiente não possui VS Code,
VS Code Insiders ou VSCodium no PATH; o guia está pronto, mas a interface
gráfica não foi validada. Os testes de encaminhamento local e SOCKS5 verificam
os mecanismos SSH usados por ferramentas remotas, sem substituir esse teste.

## Validação pública final do SSH

Após a ativação pelo operador, o cliente neste computador acessou diretamente
o domínio público na porta 443. Não foi usado o encaminhamento administrativo
SSH do preflight, nem CA privada no cliente público. O certificado público e a
identidade OpenSSH continuaram sendo verificados.

Foram confirmados autenticação por chave, comando remoto, entrada/saída binária
de 1 MiB, PTY, quatro sessões simultâneas, upload/download por SFTP de 1 MiB com
SHA-256 idêntico, encaminhamento local e SOCKS5, e sessão utilizável durante
65 segundos com keepalive. O modo com porta TCP local também abriu SSH real
pelo mesmo endpoint público. Credencial de transporte inválida foi rejeitada
com HTTP 401. O alias SSH privado foi preparado para uso no computador.

O serviço permaneceu ativo, `/probe` foi preservado, a API respondeu UP e a rota
de infraestrutura respondeu normalmente após a ativação. Os testes criaram
apenas arquivo remoto temporário do usuário, removido ao finalizar. Os clientes
auxiliares e listeners temporários também foram encerrados.

Na etapa anterior, a rede institucional e a interface gráfica do VS Code ainda
não tinham sido testadas. A validação da rede foi concluída depois, conforme o
registro abaixo. O VS Code Remote SSH segue sem validação gráfica porque o
aplicativo não estava disponível neste ambiente.

## Validação na rede do IF

Na rede do IF, o alias SSH `pagefy-wss` abriu uma sessão para a VPS pela borda
WSS/TLS na porta 443. Foram executados comandos remotos, listados os arquivos
de `/opt/pagefy` e consultados os serviços Docker. O usuário precisou de `sudo`
para acessar o socket Docker; isso foi uma permissão local do host e não uma
falha do túnel. A listagem mostrou o Nginx de borda saudável e publicado nas
portas 80 e 443.

SCP funcionou nos dois sentidos. Um arquivo de teste de 50 bytes retornou com
SHA-256 idêntico ao original. Em seguida, um arquivo de 10 MiB foi enviado em
11 s (aprox. 923 KB/s) e baixado de volta em 43 s (aprox. 235 KB/s); os hashes
SHA-256 também foram idênticos. A sessão SSH foi deixada ociosa e depois usada
novamente com sucesso.

Isso confirma, na rede testada do IF, o acesso SSH pela porta 443 e a integridade
das transferências SCP observadas. As taxas são medições daquela execução e
podem variar com as condições da rede. A interface gráfica do VS Code Remote
SSH ainda não foi validada.
