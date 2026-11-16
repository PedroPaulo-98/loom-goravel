---
title: Ícones
section: Personalização
lead: O Loom traz o Heroicons embutido. Basta usar o nome do ícone.
---

## Usando um ícone

Em qualquer lugar que aceite ícone (resources, páginas, stats), passe o nome do [Heroicons](https://heroicons.com):

```go
func (Resource) Icon() string { return "truck" }
```

## Estilos

| Valor | Estilo |
|---|---|
| `"users"` | outline (padrão) |
| `"heroicon-o-users"` ou `"o-users"` | outline |
| `"heroicon-s-users"` ou `"s-users"` | solid |

São os mesmos nomes usados no Filament, então dá para copiar de um projeto Laravel.

## Ícones disponíveis

São 324 ícones em cada estilo (Heroicons v2.2.0, 24px). Para listar:

```bash
ls packages/loom/heroicons/outline
```

Alguns úteis para painéis:

| Assunto | Ícones |
|---|---|
| Pessoas | `users`, `user`, `user-group`, `identification`, `academic-cap` |
| Lugares | `map-pin`, `home`, `building-office`, `building-storefront`, `globe-americas` |
| Documentos | `document-text`, `folder`, `paper-clip`, `clipboard-document-list`, `archive-box` |
| Dinheiro | `banknotes`, `credit-card`, `currency-dollar`, `receipt-percent`, `shopping-cart` |
| Relatórios | `chart-bar`, `chart-pie`, `presentation-chart-line`, `table-cells` |
| Sistema | `cog-6-tooth`, `shield-check`, `key`, `bell`, `wrench-screwdriver` |
| Transporte | `truck`, `rocket-launch`, `map` |
| Agenda | `calendar`, `calendar-days`, `clock` |

## Ícone inválido

Se o nome não existir, o app **não sobe** e mostra qual resource ou página usa o ícone errado:

```
loom: resource "cars" uses unknown heroicon "car" (see https://heroicons.com)
```

## Usando em templates

Em widgets e páginas que renderizam HTML com os templates do Loom, a função `icon` está disponível:

```html
{{icon "check-circle" "h-5 w-5 text-success-500"}}
```

No seu próprio código Go:

```go
import "goravel/packages/loom/heroicons"

svg := heroicons.SVG("check-circle", "h-5 w-5") // template.HTML
```
