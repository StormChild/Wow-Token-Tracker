package config

import (
	"errors"

	"github.com/danieljoos/wincred"
	"golang.org/x/sys/windows"
)

type winCred struct{}

// WinCred stores secrets as Windows Credential Manager generic credentials for the current user.
func WinCred() SecretStore { return winCred{} }

func (winCred) Get(name string) (string, error) {
	c, err := wincred.GetGenericCredential(name)
	if errors.Is(err, windows.ERROR_NOT_FOUND) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(c.CredentialBlob), nil
}

func (winCred) Set(name, value string) error {
	c := wincred.NewGenericCredential(name)
	c.UserName = "WowTokenTracker"
	c.CredentialBlob = []byte(value)
	c.Persist = wincred.PersistLocalMachine
	return c.Write()
}

func (winCred) Delete(name string) error {
	c, err := wincred.GetGenericCredential(name)
	if errors.Is(err, windows.ERROR_NOT_FOUND) {
		return nil
	}
	if err != nil {
		return err
	}
	return c.Delete()
}
