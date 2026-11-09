# Segurança

## Relatar uma vulnerabilidade

Não publique credenciais ou detalhes de exploração contra uma implantação real
em uma issue pública. Use a opção **Report a vulnerability** na aba Security do
GitHub, se ela estiver habilitada. Caso não esteja, abra apenas uma issue pedindo
um canal privado de contato, sem detalhes sensíveis.

Inclua a versão ou commit afetado, o comportamento esperado, o impacto e uma
reprodução local com dados fictícios. Não envie tokens ou chaves privadas.

## Modelo de acesso

- TLS valida certificado e hostname; não há opção para ignorar validação.
- Um token compartilhado autoriza o transporte antes do upgrade WebSocket.
- O OpenSSH mantém a autenticação do usuário e a verificação da chave do host.
- O cliente não escolhe o destino remoto: ele é fixado em loopback no servidor.
- Cada endpoint limita sessões simultâneas; os frames têm tamanho limitado.

A troca do token impede novas conexões com o valor anterior, mas não encerra
sessões já abertas. Não há revogação individual, rate limit por IP ou renovação
automática do certificado interno. Uma queda exige uma nova sessão SSH.
Esses limites e os resultados observados estão no [README](README.md) e no
[registro de validação](docs/validation-results.md).
