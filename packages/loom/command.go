package loom

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

// MakeUserCommand creates a panel user (like `php artisan make:filament-user`).
//
//	./artisan loom:make-user --name="Ana" --email=ana@example.com
type MakeUserCommand struct {
	services   func() Services
	table      string
	attributes map[string]any
}

// NewMakeUserCommand builds the command. attributes are extra columns set on the
// new user, e.g. map[string]any{"role": "admin"} so CanAccess lets them in.
func NewMakeUserCommand(services func() Services, attributes map[string]any) *MakeUserCommand {
	return &MakeUserCommand{services: services, table: "users", attributes: attributes}
}

func (c *MakeUserCommand) Signature() string { return "loom:make-user" }
func (c *MakeUserCommand) Description() string {
	return "Create a user that can access the admin panel"
}

func (c *MakeUserCommand) Extend() command.Extend {
	return command.Extend{
		Category: "loom",
		Flags: []command.Flag{
			&command.StringFlag{Name: "name", Usage: "User name"},
			&command.StringFlag{Name: "email", Usage: "User e-mail"},
			&command.StringFlag{Name: "password", Usage: "Password (prompted when omitted)"},
		},
	}
}

func (c *MakeUserCommand) Handle(ctx console.Context) error {
	notEmpty := func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("required")
		}
		return nil
	}
	validEmail := func(s string) error {
		if _, err := mail.ParseAddress(s); err != nil {
			return errors.New("invalid e-mail")
		}
		return nil
	}

	name := ctx.Option("name")
	if name == "" {
		var err error
		if name, err = ctx.Ask("Name", console.AskOption{Validate: notEmpty}); err != nil {
			return err
		}
	}
	email := ctx.Option("email")
	if email == "" {
		var err error
		if email, err = ctx.Ask("E-mail", console.AskOption{Validate: validEmail}); err != nil {
			return err
		}
	} else if err := validEmail(email); err != nil {
		ctx.Error(err.Error())
		return nil
	}
	password := ctx.Option("password")
	if password == "" {
		var err error
		if password, err = ctx.Secret("Password", console.SecretOption{Validate: notEmpty}); err != nil {
			return err
		}
	}

	svc := c.services()
	exists, err := svc.DB.Table(c.table).Where("email", email).Exists()
	if err != nil {
		return err
	}
	if exists {
		ctx.Error("A user with this e-mail already exists.")
		return nil
	}

	hashed, err := svc.Hash.Make(password)
	if err != nil {
		return err
	}
	now := time.Now()
	data := map[string]any{
		"name":       strings.TrimSpace(name),
		"email":      strings.TrimSpace(email),
		"password":   hashed,
		"created_at": now,
		"updated_at": now,
	}
	for k, v := range c.attributes {
		data[k] = v
	}
	if _, err := svc.DB.Table(c.table).Insert(data); err != nil {
		return err
	}

	ctx.Success("Admin user " + email + " created.")
	return nil
}
