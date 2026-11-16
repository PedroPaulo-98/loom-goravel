package user

import "goravel/packages/loom/table"

// Table defines the list page.
func (Resource) Table(t *table.Table) {
	t.Columns(
		table.Image("avatar").Label("Foto"),
		table.Text("id").Label("ID").Sortable(),
		table.Text("name").Label("Nome").Sortable().Searchable(),
		table.Text("email").Label("E-mail").Sortable().Searchable(),
		table.Badge("role", map[string]string{"admin": "primary", "user": "gray"}).Label("Perfil").
			FormatUsing(func(v any, _ table.Row) string {
				if table.Stringify(v) == "admin" {
					return "Administrador"
				}
				return "Usuário"
			}),
		table.Boolean("is_active").Label("Ativo"),
		table.FileLink("contract").Label("Contrato"),
		table.DateTime("created_at").Label("Criado em").Sortable(),
	).Filters(
		table.SelectFilter("role").Label("Perfil").
			Option("admin", "Administrador").
			Option("user", "Usuário"),
		table.SelectFilter("is_active").Label("Status").
			Option("true", "Ativo").
			Option("false", "Inativo"),
	).DefaultSort("created_at", "desc")
}
