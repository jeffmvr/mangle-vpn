package host

import "os"

// WriteFile writes content to path with the given permissions, creating the
// file when it does not exist and truncating it when it does. Unlike
// os.WriteFile, the permissions are applied to an existing file too.
func WriteFile(path, content string, perm os.FileMode) error {
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return err
	}
	// WriteFile only applies perm when it creates the file, so set it again
	// to cover the overwrite case.
	return os.Chmod(path, perm)
}
