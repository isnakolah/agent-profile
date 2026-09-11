//go:build darwin && cgo

package credentials

import (
	"fmt"
	keychain "github.com/keybase/go-keychain"
)

func Get(root, profile, provider string) (string, error) {
	b, e := keychain.GetGenericPassword(service, account(root, profile, provider), "", "")
	if e != nil || len(b) == 0 {
		return "", fmt.Errorf("API key unavailable; unlock Keychain or set the profile key")
	}
	return string(b), nil
}
func Set(root, profile, provider, value string) error {
	a := account(root, profile, provider)
	item := keychain.NewGenericPassword(service, a, "Agent Profile · "+profile+" · "+provider, []byte(value), "")
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlocked)
	e := keychain.AddItem(item)
	if e == keychain.ErrorDuplicateItem {
		q := keychain.NewItem()
		q.SetSecClass(keychain.SecClassGenericPassword)
		q.SetService(service)
		q.SetAccount(a)
		u := keychain.NewItem()
		u.SetData([]byte(value))
		return keychain.UpdateItem(q, u)
	}
	return e
}
func Delete(root, profile, provider string) error {
	e := keychain.DeleteGenericPasswordItem(service, account(root, profile, provider))
	if e == keychain.ErrorItemNotFound {
		return nil
	}
	return e
}
