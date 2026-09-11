//go:build linux

package credentials

import (
	"errors"
	"github.com/keybase/go-keychain/secretservice"
)

func vault() (*secretservice.SecretService, *secretservice.Session, error) {
	s, e := secretservice.NewService()
	if e != nil {
		return nil, nil, e
	}
	session, e := s.OpenSession(secretservice.AuthenticationDHAES)
	return s, session, e
}
func attrs(root, profile, provider string) secretservice.Attributes {
	return secretservice.Attributes{"service": service, "account": account(root, profile, provider)}
}
func Get(root, profile, provider string) (string, error) {
	s, x, e := vault()
	if e != nil {
		return "", e
	}
	defer s.CloseSession(x)
	items, e := s.SearchCollection(secretservice.DefaultCollection, attrs(root, profile, provider))
	if e != nil {
		return "", e
	}
	if len(items) != 1 {
		return "", errors.New("API key unavailable; unlock Secret Service or set the profile key")
	}
	b, e := s.GetSecret(items[0], *x)
	return string(b), e
}
func Set(root, profile, provider, value string) error {
	s, x, e := vault()
	if e != nil {
		return e
	}
	defer s.CloseSession(x)
	v, e := x.NewSecret([]byte(value))
	if e != nil {
		return e
	}
	_, e = s.CreateItem(secretservice.DefaultCollection, secretservice.NewSecretProperties("Agent Profile · "+profile+" · "+provider, attrs(root, profile, provider)), v, secretservice.ReplaceBehaviorReplace)
	return e
}
func Delete(root, profile, provider string) error {
	s, x, e := vault()
	if e != nil {
		return e
	}
	defer s.CloseSession(x)
	items, e := s.SearchCollection(secretservice.DefaultCollection, attrs(root, profile, provider))
	if e != nil {
		return e
	}
	for _, item := range items {
		if e = s.DeleteItem(item); e != nil {
			return e
		}
	}
	return nil
}
