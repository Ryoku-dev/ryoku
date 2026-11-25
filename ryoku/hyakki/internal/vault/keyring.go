package vault

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/godbus/dbus/v5"
)

// Keeper holds the vault's key outside the vault file. The daemon uses the
// login keyring; tests use one in memory.
type Keeper interface {
	// Key returns the 32-byte vault key. create makes a key when the keeper
	// has none; prompt lets the keeper ask the user to unlock it. An error
	// other than the vault's own reads as no keyring.
	Key(ctx context.Context, create, prompt bool) ([]byte, error)
}

// Why a keeper could not hand over the key.
var (
	// errNoKeyring is the absence of a login keyring to keep the key in.
	errNoKeyring = errors.New("vault: no login keyring")
	// errLocked is a locked keyring the user did not unlock.
	errLocked = errors.New("vault: login keyring locked")
	// errNoKey is a keyring that holds no vault key yet.
	errNoKey = errors.New("vault: no vault key")
)

// The Secret Service names the vault key lives under. A plain session sends
// the key in the clear over the user's own session bus, which is the bus the
// keyring itself trusts.
const (
	secretsName       = "org.freedesktop.secrets"
	secretsPath       = dbus.ObjectPath("/org/freedesktop/secrets")
	defaultCollection = dbus.ObjectPath("/org/freedesktop/secrets/aliases/default")
	ifaceService      = "org.freedesktop.Secret.Service"
	ifaceCollection   = "org.freedesktop.Secret.Collection"
	ifaceItem         = "org.freedesktop.Secret.Item"
	ifacePrompt       = "org.freedesktop.Secret.Prompt"
	noPrompt          = dbus.ObjectPath("/")
	keyLabel          = "Rashin vault key"
	keySize           = 32
)

// keyAttributes find the vault key among the keyring's items.
var keyAttributes = map[string]string{"application": "rashin", "purpose": "vault-key"}

// secret is the Secret Service's (oayays) secret: the session, algorithm
// parameters, the value and its content type.
type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

// secretService keeps the key as an item of the default collection of the
// login keyring. It never writes the key anywhere else.
type secretService struct{}

func (secretService) Key(ctx context.Context, create, prompt bool) ([]byte, error) {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, errNoKeyring
	}
	defer conn.Close()
	service := conn.Object(secretsName, secretsPath)
	var algorithmOutput dbus.Variant
	var session dbus.ObjectPath
	if err := service.CallWithContext(ctx, ifaceService+".OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&algorithmOutput, &session); err != nil {
		return nil, errNoKeyring
	}
	var unlocked, locked []dbus.ObjectPath
	if err := service.CallWithContext(ctx, ifaceService+".SearchItems", 0, keyAttributes).Store(&unlocked, &locked); err != nil {
		return nil, errNoKeyring
	}
	switch {
	case len(unlocked) > 0:
		return readKey(ctx, service, session, unlocked[0])
	case len(locked) > 0:
		if !prompt {
			return nil, errLocked
		}
		if err := unlock(ctx, conn, service, locked[0]); err != nil {
			return nil, err
		}
		return readKey(ctx, service, session, locked[0])
	case !create:
		return nil, errNoKey
	}
	return createKey(ctx, conn, service, session, prompt)
}

// readKey fetches the key from one item. The keyring stores it as hex text,
// so a keyring browser shows it as text rather than as raw bytes.
func readKey(ctx context.Context, service dbus.BusObject, session, item dbus.ObjectPath) ([]byte, error) {
	var secrets map[dbus.ObjectPath]secret
	if err := service.CallWithContext(ctx, ifaceService+".GetSecrets", 0, []dbus.ObjectPath{item}, session).Store(&secrets); err != nil {
		return nil, errLocked
	}
	found, ok := secrets[item]
	if !ok {
		return nil, errLocked
	}
	key, err := hex.DecodeString(string(found.Value))
	if err != nil || len(key) != keySize {
		return nil, errNoKey
	}
	return key, nil
}

// createKey makes a new key and stores it in the default collection,
// unlocking the collection first when the user may be asked.
func createKey(ctx context.Context, conn *dbus.Conn, service dbus.BusObject, session dbus.ObjectPath, prompt bool) ([]byte, error) {
	collection := conn.Object(secretsName, defaultCollection)
	lockedProp, err := collection.GetProperty(ifaceCollection + ".Locked")
	if err != nil {
		return nil, errNoKeyring
	}
	if isLocked, _ := lockedProp.Value().(bool); isLocked {
		if !prompt {
			return nil, errLocked
		}
		if err := unlock(ctx, conn, service, defaultCollection); err != nil {
			return nil, err
		}
	}
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	properties := map[string]dbus.Variant{
		ifaceItem + ".Label":      dbus.MakeVariant(keyLabel),
		ifaceItem + ".Attributes": dbus.MakeVariant(keyAttributes),
	}
	value := secret{Session: session, Parameters: []byte{}, Value: []byte(hex.EncodeToString(key)), ContentType: "text/plain"}
	var item, itemPrompt dbus.ObjectPath
	if err := collection.CallWithContext(ctx, ifaceCollection+".CreateItem", 0, properties, value, true).Store(&item, &itemPrompt); err != nil {
		return nil, errNoKeyring
	}
	if itemPrompt != noPrompt {
		if !prompt {
			return nil, errLocked
		}
		if err := runPrompt(ctx, conn, itemPrompt); err != nil {
			return nil, err
		}
	}
	return key, nil
}

// unlock asks the keyring to unlock one object, following the prompt it
// returns when the user has to type the keyring's password.
func unlock(ctx context.Context, conn *dbus.Conn, service dbus.BusObject, object dbus.ObjectPath) error {
	var unlocked []dbus.ObjectPath
	var promptPath dbus.ObjectPath
	if err := service.CallWithContext(ctx, ifaceService+".Unlock", 0, []dbus.ObjectPath{object}).Store(&unlocked, &promptPath); err != nil {
		return errLocked
	}
	if promptPath == noPrompt {
		if len(unlocked) == 0 {
			return errLocked
		}
		return nil
	}
	return runPrompt(ctx, conn, promptPath)
}

// runPrompt shows a keyring prompt and waits for the user to complete or
// dismiss it. A dismissed prompt leaves the keyring locked.
func runPrompt(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) error {
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	if err := conn.AddMatchSignalContext(ctx, dbus.WithMatchObjectPath(path), dbus.WithMatchInterface(ifacePrompt), dbus.WithMatchMember("Completed")); err != nil {
		return errLocked
	}
	if err := conn.Object(secretsName, path).CallWithContext(ctx, ifacePrompt+".Prompt", 0, "").Err; err != nil {
		return errLocked
	}
	for {
		select {
		case <-ctx.Done():
			return errLocked
		case signal, open := <-signals:
			if !open {
				return errLocked
			}
			if signal.Path != path || signal.Name != ifacePrompt+".Completed" || len(signal.Body) == 0 {
				continue
			}
			if dismissed, _ := signal.Body[0].(bool); dismissed {
				return errLocked
			}
			return nil
		}
	}
}
