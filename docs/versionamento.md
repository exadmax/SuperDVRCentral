# Versionamento e releases

O projeto segue [SemVer](https://semver.org/lang/pt-BR/): tags no formato **`vMAIOR.MENOR.PATCH`**.

| Número | Quando sobe | Exemplo |
|---|---|---|
| **PATCH** (`v1.0.1`) | correção ou ajuste pequeno, sem mudar o comportamento | corrigir um aviso, um texto, um bug |
| **MENOR** (`v1.1.0`) | funcionalidade nova que não quebra nada | nova tela, nova integração |
| **MAIOR** (`v2.0.0`) | quebra de compatibilidade | mudança no banco ou na configuração que exige ação manual |

Pré-lançamentos usam sufixo: `v1.1.0-rc.1`, `v1.1.0-beta.1`. O GitHub os marca como *pre-release*.

## Regras
- A tag sai **sempre da `main`**, com `go vet`, `go test` e `node scripts/check-i18n.js` passando.
- A tag é **imutável**. Se errou, publique o próximo PATCH; não mova nem apague tag já publicada.
- Use tag **anotada**: `git tag -a v1.0.0 -m "Central Diogo 1.0.0"`.
- Uma release por tag. As notas separam *Novidades*, *Correções* e *Atenção ao atualizar* (mudanças que pedem ação manual).

## Como publicar
```bash
git switch main && git pull
git tag -a v1.0.0 -m "Central Diogo 1.0.0"
git push origin v1.0.0
```
O workflow `.github/workflows/release.yml` confere que a tag está na `main`, roda os testes, compila os 4 binários Linux,
monta o `centraldiogo.tgz` (Linux) e o `centraldiogo-windows.zip` (instalador em janela), gera o `SHA256SUMS` e cria a release com as notas geradas pelo GitHub.

## Versão instalada no servidor
- **Pacote de release** (`./package.sh vX.Y.Z`): leva um arquivo `RELEASE`, e o `install.sh` grava essa versão em `/opt/centraldiogo/VERSION`.
  A versão do servidor passa a ser a da tag.
- **Pacote montado do código** (`./package.sh`, sem tag): não tem `RELEASE`, e cada instalação concluída soma 1 ao PATCH, como antes.
- Cada instalação, de qualquer tipo, continua registrando a linha `V<antiga> -> V<nova>` em `install.log`.

## O que vai no pacote
O pacote público usa o **`hub.config.example.json`**, nunca o `hub.config.json` local (que tem as URLs privadas das agendas).
Um pacote pessoal com a sua configuração só se gera à mão, com `CONFIG=hub.config.json ./package.sh`, e **não deve ser publicado**.
