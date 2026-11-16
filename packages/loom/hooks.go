package loom

import (
	"errors"

	contractsdb "github.com/goravel/framework/contracts/database/db"
)

// Page hooks: optional methods a Resource can implement to customize its
// list, create and edit pages (like the ListRecords, CreateRecord and
// EditRecord page classes of Filament). `loom:make-resource` writes them,
// commented out, in the resource's pages.go.

// ModifiesListQuery changes the list page query (scopes, joins, hidden rows).
type ModifiesListQuery interface {
	ModifyListQuery(ctx Context, query contractsdb.Query) contractsdb.Query
}

// BeforeCreateHook runs after validation, right before the INSERT. It may
// change data. Return Halt("message") to stop and show the message on the form.
type BeforeCreateHook interface {
	BeforeCreate(ctx Context, data map[string]any) error
}

// AfterCreateHook runs after the INSERT.
type AfterCreateHook interface {
	AfterCreate(ctx Context, data map[string]any) error
}

// BeforeSaveHook runs on the edit page after validation, right before the UPDATE.
type BeforeSaveHook interface {
	BeforeSave(ctx Context, id string, data map[string]any) error
}

// AfterSaveHook runs on the edit page after the UPDATE.
type AfterSaveHook interface {
	AfterSave(ctx Context, id string, data map[string]any) error
}

// BeforeDeleteHook runs before a single or bulk delete. Return Halt("message")
// to cancel it and show the message.
type BeforeDeleteHook interface {
	BeforeDelete(ctx Context, ids []any) error
}

// HaltError stops a page action and shows Message to the user instead of an error page.
type HaltError struct{ Message string }

func (e *HaltError) Error() string { return e.Message }

// Halt stops the current create/save/delete and shows message to the user.
func Halt(message string) error { return &HaltError{Message: message} }

func haltMessage(err error) (string, bool) {
	var h *HaltError
	if errors.As(err, &h) {
		return h.Message, true
	}
	return "", false
}
