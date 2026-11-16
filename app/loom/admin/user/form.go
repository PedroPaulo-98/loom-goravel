package user

import "goravel/packages/loom/form"

// Form defines the create/edit form.
func (Resource) Form(f *form.Form) {
	f.Schema(
		form.Text("name").Label("Nome").Required().Rules("max:255"),
		form.Email("email").Label("E-mail").Required().Unique(),
		form.Password("password").Label("Senha").Required().Rules("min:8"),
		form.Select("role").Label("Perfil").Required().
			Option("admin", "Administrador").
			Option("user", "Usuário").
			Default("user"),
		form.Toggle("is_active").Label("Ativo").Default("true"),

		form.FileUpload("avatar").Label("Foto").
			Image().Directory("avatars").MaxSize(2048).
			Help("Arquivo físico em storage/app/avatars; a coluna guarda o caminho."),
		form.FileUpload("document").Label("Documento (base64)").
			StoreAsBase64().AcceptedFileTypes("application/pdf", "image/*").MaxSize(1024),
		form.FileUpload("contract").Label("Contrato (bytea)").
			StoreAsBinary().AcceptedFileTypes("application/pdf").MaxSize(4096),
	)
}
