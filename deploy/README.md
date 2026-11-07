# Diagnóstico em uma VPS com Nginx existente

A aplicação é um diagnóstico WSS, sem encaminhamento SSH nesta etapa.
O Nginx mantém a porta pública 443, o domínio e o certificado existentes. O Go
escuta em endereço interno na porta 18443 com TLS próprio. O Nginx verifica esse
certificado. Não exponha 18443 na interface pública.

## Preparar como usuário comum

- Compilar `./cmd/probe` para o sistema/arquitetura da VPS.
- Instalar o binário em `~/.local/share/go-ws-tunnel/bin/probe`.
- Criar `~/.config/go-ws-tunnel/` com modo 0700.
- Instalar ali `access.token`, `server.key` e `server.pem`. Token e chave: 0600.
- Gerar um certificado interno com SAN `DNS:go-ws-tunnel.internal`. Escolher uma
  validade curta e registrar a expiração; a aplicação não renova certificados.
- Criar `probe.env` nesse diretório: `PROBE_LISTEN=ENDERECO_INTERNO:18443`.
- Copiar `probe-user.service` para `~/.config/systemd/user/go-ws-tunnel-probe.service`.
- Executar `systemctl --user daemon-reload` e `systemctl --user enable --now go-ws-tunnel-probe`.

Se o Nginx roda em container, localhost nele é o próprio container. O endereço
interno deve ser alcançável pelo Nginx e aceito pelo firewall do host. Não altere
regras existentes sem verificar o caminho real da conexão.

O serviço é por usuário. Sua permanência após logout/reboot depende de linger:
`loginctl show-user "$USER" -p Linger`. Um operador pode habilitá-lo com
`sudo loginctl enable-linger USUARIO` quando essa permanência for desejada.
Não há alteração automática de linger, firewall, Docker ou serviços existentes.

## Integrar à borda: ação do operador

1. Guardar backup da configuração HTTPS vigente fora do padrão `*.conf`.
2. Montar/copiar somente o certificado público interno para o caminho indicado
   em `proxy_ssl_trusted_certificate`; não montar a chave privada no Nginx.
3. Adaptar `probe-nginx.location.example` ao endereço interno real e inserir
   o bloco no servidor HTTPS escolhido. Manter rotas e certificados existentes.
4. Executar `nginx -t` dentro do ambiente real do Nginx.
5. Apenas após sucesso, executar `nginx -s reload` no mesmo ambiente.
6. Verificar as rotas de saúde existentes e executar o cliente de diagnóstico
   pelo domínio público, sem `-ca` quando a borda usa certificado público válido.
7. Restaurar o backup e recarregar caso a validação falhe.

Nenhum token deve estar no Nginx ou em URLs. O header é encaminhado sobre TLS.
O exemplo desativa access log apenas para `/probe`; o restante do serviço mantém
seu logging. Timeout do proxy precisa ser maior que o intervalo do diagnóstico.

A integração com Nginx usa [upgrade WebSocket](https://nginx.org/en/docs/http/websocket.html)
e [verificação TLS de upstream](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_verify).

## Operação e remoção

- Estado: `systemctl --user status go-ws-tunnel-probe`.
- Logs: `journalctl --user -u go-ws-tunnel-probe`.
- Renovação interna: substituir certificado e chave, atualizar o certificado
  confiável no Nginx, testar e reiniciar Go/recarregar Nginx durante manutenção.
- Token: substituir o arquivo de forma atômica, atualizar o cliente; sessões
  já abertas não são revogadas automaticamente.
- Remoção: retirar a rota Nginx, validar/recarregar e executar
  `systemctl --user disable --now go-ws-tunnel-probe`.

As configurações reais de VPS, domínio, endereços e credenciais ficam fora do Git.
