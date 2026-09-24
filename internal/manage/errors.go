package manage

import (
	"errors"
	"io/fs"
	"syscall"
)

// ReadonlyMessage explains how to make a library writable. It is the message of every
// *ReadonlyError.
const ReadonlyMessage = "The music folder is read-only for Rainy, so files cannot be changed. " +
	"Mount the library volume read-write (for example \"/volume1/music:/music\" without \":ro\") " +
	"and make sure the container user can write it: set PUID/PGID in docker-compose.yml to the " +
	"user and group that own the music files on the NAS."

// ReadonlyError reports that a file operation failed because the library (or a file in
// it) is read-only or not writable by the server process.
type ReadonlyError struct {
	Path string // library-relative path of the file that could not be written ("" if unknown)
	Err  error
}

func (e *ReadonlyError) Error() string {
	if e.Path != "" {
		return ReadonlyMessage + " (" + e.Path + ")"
	}
	return ReadonlyMessage
}

func (e *ReadonlyError) Unwrap() error { return e.Err }

// IsReadonly reports whether err is caused by a read-only file system or missing
// permissions (EROFS, EACCES, EPERM, Windows "access denied").
func IsReadonly(err error) bool {
	if err == nil {
		return false
	}
	var re *ReadonlyError
	if errors.As(err, &re) {
		return true
	}
	return errors.Is(err, syscall.EROFS) || errors.Is(err, fs.ErrPermission) ||
		errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM)
}

// fsErr converts permission / read-only errors into *ReadonlyError (rel names the file)
// and returns other errors unchanged.
func fsErr(rel string, err error) error {
	if err == nil {
		return nil
	}
	var re *ReadonlyError
	if errors.As(err, &re) {
		if re.Path == "" {
			re.Path = rel
		}
		return err
	}
	if IsReadonly(err) {
		return &ReadonlyError{Path: rel, Err: err}
	}
	return err
}

// errorText is the message stored in ItemError.Error.
func errorText(err error) string {
	var re *ReadonlyError
	if errors.As(err, &re) {
		return re.Error()
	}
	return err.Error()
}
