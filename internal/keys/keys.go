// Package keys creates and reads SSH keys. Keys are generated in-process,
// so ssh-keygen doesn't need to be installed.
package keys

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
)

// Info describes a key pair on disk.
type Info struct {
	Path        string // private key; may be missing when the key lives in an agent
	PubPath     string
	Type        string // ssh-ed25519, ssh-rsa, ...
	Fingerprint string // SHA256:...
	Public      string // "ssh-ed25519 AAAA..." without the comment
	Comment     string
	Encrypted   bool // protected by a passphrase
	HasPrivate  bool
	Created     time.Time
}

// Line is the text you paste into a host's SSH key settings.
func (i *Info) Line() string {
	if i.Comment != "" {
		return i.Public + " " + i.Comment
	}
	return i.Public
}

// ShortType is ed25519, rsa and so on.
func (i *Info) ShortType() string {
	t := strings.TrimPrefix(i.Type, "ssh-")
	if strings.HasPrefix(t, "ecdsa") {
		return "ecdsa"
	}
	if strings.HasPrefix(t, "sk-") {
		return "security key"
	}
	return t
}

// Generate creates a new key pair at path (and path.pub). typ is "ed25519"
// or "rsa". It also returns the private key so it can be handed to an
// ssh-agent without asking for the passphrase again.
func Generate(path, typ, comment string, passphrase []byte) (*Info, crypto.PrivateKey, error) {
	if fsx.Exists(path) || fsx.Exists(path+".pub") {
		return nil, nil, fmt.Errorf("%s already exists", path)
	}
	var priv crypto.PrivateKey
	var pub crypto.PublicKey
	switch typ {
	case "", "ed25519":
		pu, pr, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		priv, pub = pr, pu
	case "rsa":
		pr, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, nil, err
		}
		priv, pub = pr, &pr.PublicKey
	default:
		return nil, nil, fmt.Errorf("unknown key type %q", typ)
	}

	var block *pem.Block
	var err error
	if len(passphrase) > 0 {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, comment, passphrase)
	} else {
		block, err = ssh.MarshalPrivateKey(priv, comment)
	}
	if err != nil {
		return nil, nil, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}
	public := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))

	if err := fsx.WriteFile(path, pem.EncodeToMemory(block), true); err != nil {
		return nil, nil, err
	}
	if err := fsx.WriteFile(path+".pub", []byte(public+" "+comment+"\n"), false); err != nil {
		_ = os.Remove(path)
		return nil, nil, err
	}
	return &Info{
		Path: path, PubPath: path + ".pub", Type: sshPub.Type(),
		Fingerprint: ssh.FingerprintSHA256(sshPub), Public: public, Comment: comment,
		Encrypted: len(passphrase) > 0, HasPrivate: true, Created: time.Now(),
	}, priv, nil
}

// Load reads the key at path, which may be the private key or its .pub.
func Load(path string) (*Info, error) {
	info := &Info{Path: path, PubPath: path + ".pub"}
	if strings.HasSuffix(path, ".pub") {
		info.PubPath, info.Path = path, strings.TrimSuffix(path, ".pub")
	}
	var pub ssh.PublicKey

	privBytes, ok, err := fsx.ReadIfExists(info.Path)
	if err != nil {
		return nil, err
	}
	if ok {
		info.HasPrivate = true
		if fi, err := os.Stat(info.Path); err == nil {
			info.Created = fi.ModTime()
		}
		signer, perr := ssh.ParsePrivateKey(privBytes)
		var missing *ssh.PassphraseMissingError
		switch {
		case perr == nil:
			pub = signer.PublicKey()
		case errors.As(perr, &missing):
			info.Encrypted = true
			pub = missing.PublicKey
		default:
			return nil, fmt.Errorf("%s isn't an SSH private key git-account can read: %v", info.Path, perr)
		}
	}

	if pubBytes, ok, err := fsx.ReadIfExists(info.PubPath); err == nil && ok {
		pk, comment, _, _, err := ssh.ParseAuthorizedKey(pubBytes)
		if err == nil {
			if pub == nil {
				pub = pk
			}
			info.Comment = comment
		}
	}
	if pub == nil {
		if !info.HasPrivate {
			return nil, fmt.Errorf("%s doesn't exist", info.Path)
		}
		return nil, fmt.Errorf("can't find the public key for %s; add %s", info.Path, filepath.Base(info.PubPath))
	}
	info.Type = pub.Type()
	info.Fingerprint = ssh.FingerprintSHA256(pub)
	info.Public = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	return info, nil
}

// Find lists key pairs directly inside dir (usually ~/.ssh).
func Find(dir string) []*Info {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []*Info
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".pub") {
			continue
		}
		info, err := Load(filepath.Join(dir, strings.TrimSuffix(name, ".pub")))
		if err != nil || !info.HasPrivate {
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Delete removes a key pair and its folder if that becomes empty.
func Delete(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + ".pub")
	_ = os.Remove(filepath.Dir(path))
}
