// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"hash"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"golang.org/x/crypto/ssh"
)

// The OpenSSH signature format (PROTOCOL.sshsig), as `ssh-keygen -Y sign`
// writes it. It is implemented here rather than imported because it is a
// hundred lines over x/crypto/ssh, which is already a dependency.
const (
	sshsigMagic   = "SSHSIG"
	sshsigVersion = 1
	sshsigArmorIn = "-----BEGIN SSH SIGNATURE-----"
	sshsigArmorUp = "-----END SSH SIGNATURE-----"
	sshsigHash    = "sha512"
)

// sshsigSignedData is the blob a key actually signs: the namespace and a hash
// of the message, never the message itself.
type sshsigSignedData struct {
	Namespace string
	Reserved  string
	HashAlg   string
	Hash      []byte
}

// sshsigBlob is the signature as it travels.
type sshsigBlob struct {
	Version   uint32
	PublicKey []byte
	Namespace string
	Reserved  string
	HashAlg   string
	Signature []byte
}

func sshsigDigest(alg string, message []byte) ([]byte, bool) {
	var h hash.Hash
	switch alg {
	case "sha512":
		h = sha512.New()
	case "sha256":
		h = sha256.New()
	default:
		return nil, false
	}
	h.Write(message)
	return h.Sum(nil), true
}

func sshsigToSign(namespace, alg string, digest []byte) []byte {
	return append([]byte(sshsigMagic), ssh.Marshal(sshsigSignedData{
		Namespace: namespace, HashAlg: alg, Hash: digest,
	})...)
}

// SignSSH signs message under namespace and returns the armored signature.
// An RSA key signs with SHA-512, since SHA-1 signatures are refused.
func SignSSH(signer ssh.Signer, namespace string, message []byte) (string, error) {
	digest, _ := sshsigDigest(sshsigHash, message)
	data := sshsigToSign(namespace, sshsigHash, digest)
	var (
		sig *ssh.Signature
		err error
	)
	if algSigner, ok := signer.(ssh.AlgorithmSigner); ok && signer.PublicKey().Type() == ssh.KeyAlgoRSA {
		sig, err = algSigner.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA512)
	} else {
		sig, err = signer.Sign(rand.Reader, data)
	}
	if err != nil {
		return "", core.Internal("signing with the ssh key").Wrap(err)
	}
	blob := append([]byte(sshsigMagic), ssh.Marshal(sshsigBlob{
		Version:   sshsigVersion,
		PublicKey: signer.PublicKey().Marshal(),
		Namespace: namespace,
		HashAlg:   sshsigHash,
		Signature: ssh.Marshal(sig),
	})...)
	return armorSSHSig(blob), nil
}

func armorSSHSig(blob []byte) string {
	enc := base64.StdEncoding.EncodeToString(blob)
	var b strings.Builder
	b.WriteString(sshsigArmorIn + "\n")
	for len(enc) > 70 {
		b.WriteString(enc[:70] + "\n")
		enc = enc[70:]
	}
	b.WriteString(enc + "\n" + sshsigArmorUp + "\n")
	return b.String()
}

func errSignature() error {
	return core.Unauthenticated("the signature does not verify for this key and challenge")
}

// VerifySSH checks an armored sshsig signature over message by key, made
// under namespace. A signature under any other namespace is refused naming the
// one expected, which is the mistake a person signing by hand makes.
func VerifySSH(key ssh.PublicKey, armored, namespace string, message []byte) error {
	text := strings.TrimSpace(armored)
	if !strings.HasPrefix(text, sshsigArmorIn) || !strings.HasSuffix(text, sshsigArmorUp) {
		return core.Invalid("the signature is not an armored ssh signature")
	}
	body := strings.Join(strings.Fields(text[len(sshsigArmorIn):len(text)-len(sshsigArmorUp)]), "")
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil || !bytes.HasPrefix(raw, []byte(sshsigMagic)) {
		return core.Invalid("the signature is not an armored ssh signature")
	}
	var blob sshsigBlob
	if err := ssh.Unmarshal(raw[len(sshsigMagic):], &blob); err != nil || blob.Version != sshsigVersion {
		return core.Invalid("the signature is not a version 1 ssh signature")
	}
	if blob.Namespace != namespace {
		return core.Unauthenticated("the signature was made for namespace %q, not %q", blob.Namespace, namespace)
	}
	if subtle.ConstantTimeCompare(blob.PublicKey, key.Marshal()) != 1 {
		return errSignature()
	}
	digest, ok := sshsigDigest(blob.HashAlg, message)
	if !ok {
		return core.Invalid("the signature uses hash %q; sha512 or sha256 is required", blob.HashAlg)
	}
	var sig ssh.Signature
	if err := ssh.Unmarshal(blob.Signature, &sig); err != nil {
		return errSignature()
	}
	if sig.Format == ssh.KeyAlgoRSA {
		return core.Unauthenticated("ssh-rsa signatures use SHA-1 and are refused; sign with rsa-sha2-512")
	}
	if err := key.Verify(sshsigToSign(namespace, blob.HashAlg, digest), &sig); err != nil {
		return errSignature()
	}
	return nil
}
