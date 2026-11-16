package form

import (
	"strings"
)

// Storage is where an uploaded file is kept.
type Storage string

const (
	// StorageDisk saves the file on a Goravel filesystem disk and stores its path in the column (varchar).
	StorageDisk Storage = "disk"
	// StorageBase64 stores the file in the column as a data URI: "data:<mime>;base64,<...>" (text).
	StorageBase64 Storage = "base64"
	// StorageBinary stores the raw bytes in the column (bytea on PostgreSQL, blob on MySQL).
	StorageBinary Storage = "binary"
)

// Upload holds the file upload settings of a field.
type Upload struct {
	Storage   Storage
	Disk      string
	Directory string
	Accepted  []string
	MaxSizeKB int64
	Image     bool
}

// FileUpload creates a file input (like Filament's FileUpload).
// By default the file is saved on the "local" disk and its path stored in the column.
//
//	form.FileUpload("avatar").Image().Directory("avatars")      // file on disk, path in varchar
//	form.FileUpload("document").StoreAsBase64()                 // data URI in a text column
//	form.FileUpload("contract").StoreAsBinary()                 // raw bytes in a bytea column
func FileUpload(name string) *Field {
	f := newField(KindFile, name)
	f.upload = &Upload{Storage: StorageDisk, Disk: "local", MaxSizeKB: 5 * 1024}
	return f
}

func (f *Field) mustUpload() *Upload {
	if f.upload == nil {
		panic("form: " + f.name + " is not a FileUpload field")
	}
	return f.upload
}

// Disk saves the file on the given Goravel filesystem disk (config/filesystems.go).
func (f *Field) Disk(disk string) *Field {
	u := f.mustUpload()
	u.Storage, u.Disk = StorageDisk, disk
	return f
}

// Directory is the folder inside the disk (disk storage only).
func (f *Field) Directory(dir string) *Field {
	f.mustUpload().Directory = strings.Trim(dir, "/")
	return f
}

// StoreAsBase64 keeps the file inside the column as a base64 data URI (text column).
func (f *Field) StoreAsBase64() *Field { f.mustUpload().Storage = StorageBase64; return f }

// StoreAsBinary keeps the raw bytes inside the column (bytea / blob column).
func (f *Field) StoreAsBinary() *Field { f.mustUpload().Storage = StorageBinary; return f }

// AcceptedFileTypes limits the MIME types, e.g. "image/*", "application/pdf".
// The type is detected from the file content, not from the name.
func (f *Field) AcceptedFileTypes(types ...string) *Field {
	u := f.mustUpload()
	u.Accepted = append(u.Accepted, types...)
	return f
}

// MaxSize limits the file size in kilobytes (default 5120 = 5 MB).
func (f *Field) MaxSize(kb int64) *Field { f.mustUpload().MaxSizeKB = kb; return f }

// Image accepts only images (png, jpeg, gif, webp) and shows a preview.
func (f *Field) Image() *Field {
	u := f.mustUpload()
	u.Image = true
	u.Accepted = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}
	return f
}

// UploadSettings returns the upload configuration (nil for non-file fields).
func (f *Field) UploadSettings() *Upload { return f.upload }

// AcceptAttr is the value of the HTML accept attribute.
func (f *Field) AcceptAttr() string {
	if f.upload == nil {
		return ""
	}
	return strings.Join(f.upload.Accepted, ",")
}

// Accepts reports whether the detected MIME type is allowed.
func (u *Upload) Accepts(mime string) bool {
	if len(u.Accepted) == 0 {
		return true
	}
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	for _, a := range u.Accepted {
		a = strings.ToLower(a)
		if a == mime || (strings.HasSuffix(a, "/*") && strings.HasPrefix(mime, strings.TrimSuffix(a, "*"))) {
			return true
		}
	}
	return false
}

// IsBlob reports whether the file content lives inside the database column.
func (u *Upload) IsBlob() bool { return u.Storage == StorageBase64 || u.Storage == StorageBinary }
