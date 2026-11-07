# Roteiro para o primeiro teste na rede de uso

Antes de sair, confirme que possui o binário `tunnel`, a chave SSH, o token e a
entrada SSH no computador. Não dependa de baixar dependências naquela rede.
Faça o teste somente quando esse tipo de acesso estiver autorizado.

1. Abra o diagnóstico pelo domínio real usando seu token:

   ```bash
   ./bin/probe client -url wss://tunel.example.com/probe \
     -token-file local/access.token -duration 30s
   ```

2. Abra uma sessão com a entrada SSH previamente preparada:

   ```bash
   ssh minha-vps-wss 'printf ssh-ok'
   ssh minha-vps-wss
   ```

3. Execute comandos e teste uma pequena transferência por SFTP/SCP.
4. Mantenha a sessão ociosa por alguns minutos, depois execute outro comando.
5. Se usar VS Code, conecte pelo mesmo alias e abra uma pasta remota.

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
