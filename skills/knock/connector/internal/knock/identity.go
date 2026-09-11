package knock

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type Identity struct {
	Certificate tls.Certificate
	Pin         string
}

func pinFor(cert *x509.Certificate) string {
	v := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return base64.RawURLEncoding.EncodeToString(v[:])
}
func validPin(pin string) bool {
	b, e := base64.RawURLEncoding.DecodeString(pin)
	return e == nil && len(b) == 32
}
func CurlPin(pin string) string {
	b, _ := base64.RawURLEncoding.DecodeString(pin)
	return "sha256//" + base64.StdEncoding.EncodeToString(b)
}
func LoadIdentity(root string) (*Identity, error) {
	p := filepath.Join(root, "identity.pem")
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return nil, e
		}
		serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if e != nil {
			return nil, e
		}
		t := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Knock device"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
		der, e := x509.CreateCertificate(rand.Reader, t, t, &key.PublicKey, key)
		if e != nil {
			return nil, e
		}
		priv, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil {
			return nil, e
		}
		b = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv})...)
		if e = writePrivate(p, b); e != nil {
			return nil, e
		}
	} else if e != nil {
		return nil, e
	}
	cert, e := tls.X509KeyPair(b, b)
	if e != nil {
		return nil, e
	}
	leaf, e := x509.ParseCertificate(cert.Certificate[0])
	if e != nil {
		return nil, e
	}
	cert.Leaf = leaf
	return &Identity{cert, pinFor(leaf)}, nil
}
func (i *Identity) ServerTLS() *tls.Config {
	// TLS verifies CertificateVerify possession even for certificates without a CA.
	// Bootstrap allows no client certificate; pairing/stream routes enforce identity.
	return &tls.Config{Certificates: []tls.Certificate{i.Certificate}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, ClientAuth: tls.RequestClientCert, SessionTicketsDisabled: true, NextProtos: []string{"http/1.1"}}
}
func (i *Identity) ClientTLS(pin string) *tls.Config {
	c := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{"http/1.1"}}
	if i != nil {
		c.Certificates = []tls.Certificate{i.Certificate}
	}
	// CA/hostname validation is replaced, never omitted, by the out-of-band SPKI pin.
	c.VerifyConnection = func(s tls.ConnectionState) error {
		if !validPin(pin) || len(s.PeerCertificates) == 0 {
			return errors.New("missing pinned identity")
		}
		cert := s.PeerCertificates[0]
		if subtle.ConstantTimeCompare([]byte(pinFor(cert)), []byte(pin)) != 1 {
			return errors.New("peer identity changed: pairing required")
		}
		if time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
			return errors.New("peer certificate is outside its validity period")
		}
		return nil
	}
	return c
}
