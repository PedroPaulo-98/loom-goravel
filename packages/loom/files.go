package loom

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	contractshttp "github.com/goravel/framework/contracts/http"

	"goravel/packages/loom/form"
	"goravel/packages/loom/table"
)

type pendingUpload struct {
	field   *form.Field
	tmpPath string
	name    string
	mime    string
}

type diskFile struct{ disk, path string }

// uploadSet tracks the files of one form submission.
type uploadSet struct {
	pending  []pendingUpload
	removed  []*form.Field // fields the user asked to clear
	current  table.Row     // record before the update (nil on create)
	stored   []diskFile    // new files written to disk (rolled back on error)
	replaced []diskFile    // old files to delete after a successful save
}

func hasFileValue(v any) bool {
	switch val := v.(type) {
	case nil:
		return false
	case string:
		return val != ""
	case []byte:
		return len(val) > 0
	case int64:
		return val != 0 // presence flag from listSelect
	case int32:
		return val != 0
	case int:
		return val != 0
	default:
		return true
	}
}

func (e *resourceEntry) fileFields() []*form.Field {
	var out []*form.Field
	for _, f := range e.form.Fields(false) {
		if f.Kind() == form.KindFile {
			out = append(out, f)
		}
	}
	return out
}

func (e *resourceEntry) hasFileFields() bool { return len(e.fileFields()) > 0 }

func (e *resourceEntry) fileField(name string) *form.Field {
	for _, f := range e.fileFields() {
		if f.Name() == name {
			return f
		}
	}
	return nil
}

// collectUploads validates the uploaded files (size, type, required) without storing them.
func (h *crudHandler) collectUploads(ctx contractshttp.Context, fields []*form.Field, current table.Row) (*uploadSet, map[string]string) {
	set := &uploadSet{current: current}
	errs := map[string]string{}
	req := ctx.Request()

	for _, f := range fields {
		if f.Kind() != form.KindFile {
			continue
		}
		u := f.UploadSettings()
		hasCurrent := current != nil && hasFileValue(current[f.Name()])
		remove := req.Input("remove_"+f.Name()) == "1"

		file, err := req.File(f.Name())
		if err != nil || file == nil { // no file sent
			switch {
			case remove && f.IsRequired():
				errs[f.Name()] = "O campo " + f.LabelText() + " é obrigatório."
			case remove && hasCurrent:
				set.removed = append(set.removed, f)
			case f.IsRequired() && !hasCurrent:
				errs[f.Name()] = "O campo " + f.LabelText() + " é obrigatório."
			}
			continue
		}

		tmp := file.File()
		size, err := file.Size()
		if err != nil {
			errs[f.Name()] = "Não foi possível ler o arquivo."
			_ = os.Remove(tmp)
			continue
		}
		if u.MaxSizeKB > 0 && size > u.MaxSizeKB*1024 {
			errs[f.Name()] = fmt.Sprintf("O arquivo deve ter no máximo %d KB.", u.MaxSizeKB)
			_ = os.Remove(tmp)
			continue
		}
		mime, err := file.MimeType() // detected from the content, not the file name
		if err != nil || !u.Accepts(mime) {
			errs[f.Name()] = "Tipo de arquivo não permitido (" + mime + ")."
			_ = os.Remove(tmp)
			continue
		}
		set.pending = append(set.pending, pendingUpload{field: f, tmpPath: tmp, name: file.GetClientOriginalName(), mime: mime})
	}
	return set, errs
}

// apply writes the files (disk) or encodes them (base64/binary) into data.
func (s *uploadSet) apply(ctx contractshttp.Context, p *Panel, data map[string]any) error {
	for _, f := range s.removed {
		data[f.Name()] = nil
		s.markReplaced(f)
	}
	for _, up := range s.pending {
		u := up.field.UploadSettings()
		content, err := os.ReadFile(up.tmpPath)
		if err != nil {
			return err
		}
		switch u.Storage {
		case form.StorageBase64:
			data[up.field.Name()] = "data:" + up.mime + ";base64," + base64.StdEncoding.EncodeToString(content)
		case form.StorageBinary:
			data[up.field.Name()] = content
		default:
			ext := ""
			if i := strings.LastIndex(up.name, "."); i >= 0 && len(up.name)-i <= 8 {
				ext = strings.ToLower(up.name[i:])
			}
			path := strings.TrimLeft(u.Directory+"/"+randomName()+ext, "/")
			if err := p.svc.Storage.Disk(u.Disk).Put(path, string(content)); err != nil {
				return err
			}
			s.stored = append(s.stored, diskFile{u.Disk, path})
			data[up.field.Name()] = path
		}
		s.markReplaced(up.field)
	}
	return nil
}

// markReplaced schedules the current disk file of f for deletion after a successful save.
func (s *uploadSet) markReplaced(f *form.Field) {
	u := f.UploadSettings()
	if s.current != nil && u.Storage == form.StorageDisk && hasFileValue(s.current[f.Name()]) {
		s.replaced = append(s.replaced, diskFile{u.Disk, table.Stringify(s.current[f.Name()])})
	}
}

func randomName() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// rollback deletes files written by apply (the database save failed).
func (s *uploadSet) rollback(p *Panel) { p.deleteDiskFiles(s.stored) }

// commit deletes files that were replaced or removed (the save succeeded).
func (s *uploadSet) commit(p *Panel) { p.deleteDiskFiles(s.replaced) }

// cleanup removes the temporary request files.
func (s *uploadSet) cleanup() {
	for _, up := range s.pending {
		_ = os.Remove(up.tmpPath)
	}
}

func (p *Panel) deleteDiskFiles(files []diskFile) {
	for _, f := range files {
		if f.path == "" {
			continue
		}
		if err := p.svc.Storage.Disk(f.disk).Delete(f.path); err != nil {
			log.Printf("loom: delete %s:%s: %v", f.disk, f.path, err)
		}
	}
}

// diskFilesOf returns the disk files referenced by the given records.
func (h *crudHandler) diskFilesOf(ctx contractshttp.Context, ids []any) ([]diskFile, error) {
	var cols []string
	settings := map[string]*form.Upload{}
	for _, f := range h.res.fileFields() {
		if u := f.UploadSettings(); u.Storage == form.StorageDisk {
			cols = append(cols, f.Name())
			settings[f.Name()] = u
		}
	}
	if len(cols) == 0 {
		return nil, nil
	}
	rows, err := fetchRows(h.panel.svc.DB.WithContext(ctx).Table(h.res.TableName()).Select(cols...).WhereIn(h.res.PrimaryKey(), ids))
	if err != nil {
		return nil, err
	}
	var files []diskFile
	for _, r := range rows {
		for _, c := range cols {
			if hasFileValue(r[c]) {
				files = append(files, diskFile{settings[c].Disk, table.Stringify(r[c])})
			}
		}
	}
	return files, nil
}

// listSelect narrows the list query when file contents live in the table:
// blob columns are replaced by a presence flag so the list never loads them.
// It returns nil (select *) when the resource has no base64/binary fields.
func (e *resourceEntry) listSelect() []string {
	blobs := map[string]bool{}
	for _, f := range e.fileFields() {
		if f.UploadSettings().IsBlob() {
			blobs[f.Name()] = true
		}
	}
	if len(blobs) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var cols []string
	add := func(c string) {
		if seen[c] {
			return
		}
		seen[c] = true
		if blobs[c] {
			cols = append(cols, "CASE WHEN "+c+" IS NULL THEN 0 ELSE 1 END AS "+c)
		} else {
			cols = append(cols, c)
		}
	}
	add(e.PrimaryKey())
	for _, c := range e.table.GetColumns() {
		add(c.Name())
	}
	for _, f := range e.table.GetFilters() {
		add(f.Name())
	}
	for _, c := range e.table.GetSelect() {
		add(c)
	}
	return cols
}

// inlineTypes may be displayed in the browser; anything else is downloaded.
var inlineTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "application/pdf": true,
}

// file streams a stored file: GET /{slug}/{id}/files/{field} (requires login).
func (h *crudHandler) file(ctx contractshttp.Context) contractshttp.Response {
	req := ctx.Request()
	f := h.res.fileField(req.Route("field"))
	if f == nil {
		return h.panel.notFound(ctx)
	}
	rows, err := fetchRows(h.panel.svc.DB.WithContext(ctx).Table(h.res.TableName()).
		Select(f.Name()).Where(h.res.PrimaryKey(), normalizeID(req.Route("id"))).Limit(1))
	if err != nil {
		return h.panel.serverError(ctx, err)
	}
	if len(rows) == 0 || !hasFileValue(rows[0][f.Name()]) {
		return h.panel.notFound(ctx)
	}

	var content []byte
	mime := ""
	value := rows[0][f.Name()]
	switch f.UploadSettings().Storage {
	case form.StorageBase64:
		s := table.Stringify(value)
		if strings.HasPrefix(s, "data:") {
			if i := strings.Index(s, ";base64,"); i > 0 {
				mime, s = s[5:i], s[i+8:]
			}
		}
		content, err = base64.StdEncoding.DecodeString(s)
	case form.StorageBinary:
		switch v := value.(type) {
		case []byte:
			content = v
		case string:
			content = []byte(v)
		}
	default:
		content, err = h.panel.svc.Storage.Disk(f.UploadSettings().Disk).GetBytes(table.Stringify(value))
	}
	if err != nil {
		return h.panel.serverError(ctx, err)
	}

	// Never trust the stored type for rendering: sniff the bytes.
	mime = strings.Split(http.DetectContentType(content), ";")[0]
	disposition := "attachment"
	if inlineTypes[mime] {
		disposition = "inline"
	}
	res := ctx.Response()
	res.Header("Content-Disposition", disposition+`; filename="`+f.Name()+extFor(mime)+`"`)
	res.Header("X-Content-Type-Options", "nosniff")
	res.Header("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; sandbox")
	res.Header("Cache-Control", "private, no-cache")
	return res.Data(200, mime, content)
}

func extFor(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	}
	return ""
}
