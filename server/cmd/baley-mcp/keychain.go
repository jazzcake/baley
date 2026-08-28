package main

import systemkeyring "github.com/zalando/go-keyring"

const credentialKeychainService = "io.baley.mcp"

// secretStore keeps device-bound MCP material out of the credential-store file.
// Tests inject a memory implementation; production uses the platform keychain
// (macOS Keychain, Windows Credential Manager, or Secret Service).
type secretStore interface {
	Get(service, key string) (string, error)
	Set(service, key, value string) error
	Delete(service, key string) error
}

type osSecretStore struct{}

func newOSSecretStore() secretStore { return osSecretStore{} }

func (osSecretStore) Get(service, key string) (string, error) {
	return systemkeyring.Get(service, key)
}

func (osSecretStore) Set(service, key, value string) error {
	return systemkeyring.Set(service, key, value)
}

func (osSecretStore) Delete(service, key string) error {
	return systemkeyring.Delete(service, key)
}
