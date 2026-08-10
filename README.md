# w2pp-OpenWYD
servidor aberto de wyd

[![CI](https://github.com/Jean1dev/w2pp-OpenWYD/actions/workflows/ci.yml/badge.svg)](https://github.com/Jean1dev/w2pp-OpenWYD/actions/workflows/ci.yml)

Reescrita em **Go** (big-bang) do servidor do WYD (With Your Destiny), mirando o **client `WYD.exe`
7662 sem modificação** (protocolo 7640). Os fontes legados em C++ ficam em `Source/`, os binários
legados + conteúdo do jogo em `Release/`, e os serviços Go novos em `tmserver/`, `dbserver/`,
`binserver/`, `webserver/`. A reescrita é guiada pela engenharia reversa documentada em
`docs/migration/`. Agradecimentos ao pessoal que me forneceu os fontes.

> Para detalhes de arquitetura, comandos e convenções, veja `CLAUDE.md`.

## Serviços

Só a borda client↔tmServer fala o protocolo legado; os links internos são gRPC (+mTLS):

- **tmServer** (`:8281` jogo + `:80` status) — servidor do jogo; dono de todo o estado do mundo em
  memória, num único goroutine sem locks (espelha o reactor single-thread original).
- **dbServer** (`:7514`) — persistência (PostgreSQL/pgx v5) via gRPC `api/db/v1`.
- **binServer** (`:3000`) — gate de billing via gRPC `api/bin/v1`.
- **webServer** (`:7600`) — web-api (gRPC `api/web/v1`) que a plataforma web (Next.js BFF) chama
  server-side; criar conta, login web e, no plano, cash/ranking/loja-web.

Pacotes compartilhados (store, migrations, domain, secret/argon2id) ficam na raiz `internal/`.

## Como rodar

```bash
make run            # só o tmserver, persistência no-op — bring-up rápido do protocolo
make run-local      # stack completa via docker compose + semeia a conta test/test123
make test           # go test -race -cover ./...
```

`make run-local` imprime o IP:porta para apontar um client Windows real. A conta começa sem
personagens — crie-os no client.

## Cliente do jogo

O servidor Go mira o **`WYD.exe` build 7662 sem modificação**, que usa o protocolo/
`ClientVersion` **7640**. O repositório não distribui um instalador oficial do cliente. Os links antigos
do servidor de teste e do cliente 7.59 foram removidos porque estão offline; veja o histórico nas
issues [#85](https://github.com/Jean1dev/w2pp-OpenWYD/issues/85) e
[#235](https://github.com/Jean1dev/w2pp-OpenWYD/issues/235).

O download do cliente está disponível em **[wyd-ten.vercel.app](https://wyd-ten.vercel.app/)**.

Use uma cópia do cliente obtida de uma fonte confiável e confira arquivos executáveis com seu
antivírus e, se possível, com um serviço como o VirusTotal. Clientes customizados de outros
servidores podem ter launcher, IP ou protocolo alterados e não são garantidos como compatíveis.

### Conectar o cliente ao servidor local

1. Inicie a stack com `make run-local` e anote o IP e a porta exibidos (por padrão, porta `8281`).
2. Abra `serverlist bin/serverlist editor.exe` e gere um `serverlist.bin` apontando para esse IP e
   porta. Para jogar na mesma máquina use o IP informado pelo script; para outra máquina na LAN,
   use um IP do servidor acessível por ela.
3. Faça backup do `serverlist.bin` existente na pasta do cliente e substitua-o pelo arquivo gerado.
   Os arquivos prontos dentro de `serverlist bin/` contêm IPs privados de exemplo e normalmente
   não servem para o seu ambiente.
4. Execute `WYD.exe`, entre com a conta local `test` / `test123` e crie um personagem.

O cliente consulta o status do servidor na porta HTTP `80` e conecta ao jogo na porta TCP `8281`;
libere ambas no firewall quando o cliente estiver em outra máquina. Se o cliente não listar o
servidor, gere novamente o `serverlist.bin` com o IP impresso por `make run-local` e confirme que as
duas portas estão acessíveis.
