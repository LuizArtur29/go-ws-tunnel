# Primeiro teste na rede do IF

O teste foi concluído na rede do IF usando o alias SSH `pagefy-wss`, já
configurado no computador. O acesso passou pelo endpoint WebSocket sobre TLS
na porta 443; a porta 22 direta não foi necessária para a conexão do cliente.

## Resultados

- SSH conectou à VPS e permitiu executar comandos remotos.
- A inspeção de `/opt/pagefy` funcionou. Para consultar Docker no host foi
  necessário `sudo`, devido às permissões do socket Docker.
- SCP enviou e trouxe de volta um arquivo de 50 bytes, com SHA-256 igual.
- SCP enviou um arquivo de 10 MiB em 11 s (aprox. 923 KB/s) e o baixou em
  43 s (aprox. 235 KB/s). O SHA-256 da origem e do arquivo retornado foi igual.
- A sessão permaneceu ociosa e aceitou um comando depois.

As taxas são observações daquela execução e podem variar conforme as condições
da rede. O teste valida SSH e SCP na rede usada; o VS Code Remote SSH ainda
precisa de uma validação própria, caso seja parte do uso desejado.

## Comandos usados

```bash
ssh pagefy-wss
scp /tmp/tunel-envio.txt pagefy-wss:/tmp/tunel-if.txt
scp pagefy-wss:/tmp/tunel-if.txt /tmp/tunel-retorno.txt
sha256sum /tmp/tunel-envio.txt /tmp/tunel-retorno.txt
```

Para repetir a transferência maior, gere um arquivo de 10 MiB e use o mesmo
padrão SCP de envio, retorno e comparação SHA-256. Não coloque tokens ou chaves
em comandos compartilhados, capturas ou logs públicos.

## Se falhar

- Diagnóstico não conecta: verificar DNS, certificado, proxy obrigatório e
  permissões de acesso ao domínio/porta. Não desabilitar verificação TLS.
- HTTP 401: conferir token do cliente; não colocá-lo na URL nem em capturas.
- HTTP 404: verificar domínio e rota. `/probe` e `/ssh` são diferentes.
- HTTP 400: conferir se cliente e servidor negociam a mesma versão do protocolo.
- HTTP 503: conferir disponibilidade da credencial e limite de sessões.
- Autenticação SSH rejeitada: investigar usuário/chave; o túnel não autentica
  automaticamente o usuário no sistema operacional.
- Chave do servidor diferente: interromper e conferir a identidade por outro canal.
- Queda de sessão: registrar duração, se houve ociosidade e comportamento do
  diagnóstico. Reiniciar o transporte não recupera uma sessão SSH antiga.

Registre commit, etapa e mensagem sanitizada. Não publique dados da rede
institucional ou credenciais. Uma falha nessa rede pode exigir adaptação ao proxy
ou às regras locais; a porta 443 aberta não garante que WebSocket seja permitido.
