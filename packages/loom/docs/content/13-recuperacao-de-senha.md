---
title: Recuperação de senha
section: Usuários
lead: O "Esqueceu a senha?" envia um link por e-mail (SMTP) para o usuário criar uma senha nova.
---

## Ativando

```go
BaseURL(facades.Config().GetString("app.url")).
PasswordReset()
```

Isso cria as telas `/admin/forgot-password` e `/admin/reset-password/{token}`, e o link **Esqueceu a senha?** no login.

Também é preciso passar o serviço `Mail` no provider:

```go
Mail: facades.Mail(),
```

## Tabela de tokens

```bash
go run . artisan make:migration create_password_reset_tokens_table
```

```go
facades.Schema().Create("password_reset_tokens", func(table schema.Blueprint) {
	table.String("email")
	table.String("token")          // hash SHA-256 do token enviado
	table.TimestampTz("created_at")
	table.Index("email")
})
```

Para usar outro nome de tabela: `PasswordResetTable("tokens_senha")`.

## Configurando o SMTP

No `.env`:

```env
APP_URL=https://sistema.exemplo.com

MAIL_HOST=smtp.seu-provedor.com
MAIL_PORT=587
MAIL_USERNAME=usuario
MAIL_PASSWORD=senha
MAIL_FROM_ADDRESS=no-reply@exemplo.com
MAIL_FROM_NAME="SIAC"
```

| Porta | Conexão |
|---|---|
| `587` | STARTTLS (a mais comum) |
| `465` | TLS direto |
| outras | sem criptografia (só para testes locais) |

> O Goravel sempre autentica no SMTP, então o servidor precisa oferecer AUTH. Gmail, Outlook, Amazon SES, SendGrid, Mailgun e afins oferecem.

## Testando localmente com Mailpit

O [Mailpit](https://mailpit.axllent.org) é uma caixa de entrada falsa: captura os e-mails e mostra numa página web.

```bash
docker run -d --name goravel-mailpit -e MP_SMTP_AUTH_ACCEPT_ANY=1 -e MP_SMTP_AUTH_ALLOW_INSECURE=1 -p 54025:1025 -p 58025:8025 axllent/mailpit:v1.21
```

```env
MAIL_HOST=127.0.0.1
MAIL_PORT=54025
MAIL_USERNAME=mailpit
MAIL_PASSWORD=mailpit
```

Os e-mails aparecem em `http://localhost:58025`.

## O fluxo

1. O usuário informa o e-mail em **Esqueceu a senha?**.
2. Se a conta existir, o Loom gera um token aleatório de 256 bits e grava **só o hash** dele.
3. O e-mail chega com o botão **Redefinir senha**, na cor `primary` do painel.
4. O link abre a tela de nova senha (com confirmação).
5. A senha é trocada, o token é apagado e o usuário volta ao login.

## Opções

```go
PasswordResetExpiry(30 * time.Minute) // validade do link (padrão 60 min)
PasswordRules("min:10")               // regras da nova senha
```

## Segurança

- A resposta é sempre "Se existir uma conta com este e-mail, enviamos um link...", **exista ou não** a conta. Assim ninguém descobre quais e-mails estão cadastrados.
- O token vale **uma vez** e expira.
- A comparação do token é feita em tempo constante.
- No máximo **um e-mail por minuto** por endereço.
- Falhas de envio vão para o log do servidor, não para a tela.

## Personalizando o e-mail

O template fica em `packages/loom/views/emails/reset.html`. Ele recebe:

| Variável | Conteúdo |
|---|---|
| `.Brand` | `BrandName` do painel |
| `.Name` | nome do usuário |
| `.Link` | link de redefinição |
| `.Minutes` | validade em minutos |
| `.Color` | hex da cor `primary` (tom 600) |
