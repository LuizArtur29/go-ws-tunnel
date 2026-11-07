# Registro de validação

Data: 7 de novembro de 2026 (America/Recife).
Etapa: diagnóstico autenticado WebSocket sobre TLS; sem encaminhamento SSH.

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
| Borda pública 443 | Configuração preparada; aplicação exige sudo pelo operador | Pendente |
| Rede institucional | Teste adiado, sem acesso à rede no momento | Não executado |

Os testes automatizados também cobrem rota incorreta, query inesperada, Origin
de navegador, header de autenticação duplicado, rotação do token, mensagem de
texto, tamanho excessivo, limite de sessões e cancelamento do servidor.

## Limites da evidência

O laboratório usou certificado de teste explicitamente confiado com `-ca`; o
cliente continuou verificando identidade e cadeia. O backend da VPS usa outro
certificado interno. A borda deve continuar usando seu certificado público.
Nenhuma chave privada, token, endereço interno real ou log bruto integra este
registro público. Os processos locais de teste foram encerrados ao terminar.

O teste interno da VPS não comprova passagem pelo proxy Nginx, pelo domínio
público ou pela rede institucional. Isso precisa de evidência separada.

## Correção da integração com a borda

A primeira ativação passou em `nginx -t`, mas a conexão pública retornou
HTTP 504. O script restaurou a configuração anterior e a API permaneceu UP.
O backend Go estava ativo. A evidência indica falha no caminho TCP entre
container e host; a regra específica de rede responsável não foi confirmada.

O backend passou a oferecer TLS sobre socket Unix compartilhado. Isso elimina
a dependência desse trecho em TCP/bridge sem alterar regras de firewall.
Testes com `-race` cobrem round-trip WSS/TLS pelo socket, modo 0600, preservação
de arquivos existentes e remoção do socket no fechamento normal.
A nova ativação pública continua pendente de execução pelo operador.
