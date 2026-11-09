# Contribuir

O go-ws-tunnel é um projeto de aprendizado com um túnel SSH funcional.
Correções, melhorias de documentação e relatos reproduzíveis são bem-vindos.
Para mudanças maiores no protocolo ou no escopo, abra uma issue para discutir
antes de implementar.

## Preparar o ambiente

Use Go 1.24 ou superior, Git e Make. Linux é o ambiente de operação documentado.

```bash
git clone https://github.com/LuizArtur29/go-ws-tunnel.git
cd go-ws-tunnel
make build
make check
```

`make check` executa go vet, testes com detector de corridas e verificação de
formatação. O CI executa essas verificações e compila os comandos.
O [laboratório local](docs/quickstart.md) permite experimentar sem uma VPS.

## Enviar uma mudança

1. Crie uma branch no seu fork.
2. Faça uma alteração com propósito definido e atualize a documentação afetada.
3. Inclua testes quando mudar comportamento do protocolo ou ciclo de vida.
4. Execute `make check` e `make build`.
5. Abra um pull request explicando o problema, o comportamento resultante e
   como verificou a mudança.

Preserve a verificação TLS, o destino fixo no servidor, o half-close e o stdout
exclusivo para bytes SSH no modo stdio. Nunca adicione tokens, chaves,
certificados privados, configurações pessoais ou logs com credenciais.
Relatos de segurança seguem [SECURITY.md](SECURITY.md).
