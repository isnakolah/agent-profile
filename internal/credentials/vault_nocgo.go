//go:build darwin && !cgo

package credentials

import "errors"

func Get(root, profile, provider string) (string, error) {
	return "", errors.New("Keychain requires a build with CGO_ENABLED=1")
}
func Set(root, profile, provider, value string) error {
	return errors.New("Keychain requires a build with CGO_ENABLED=1")
}
func Delete(root, profile, provider string) error {
	return errors.New("Keychain requires a build with CGO_ENABLED=1")
}
