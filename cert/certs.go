// package cert provides tools for use with cmd/9cert, used to create and use
// certificates for encryption and authentication.
package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"time"
)

// Generate a certificate authority and write it in PEM format to cafile
func GenCA(cafile string) error {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization: []string{"go9p"},
			//Country:       []string{"US"},
			//Province:      []string{""},
			//Locality:      []string{"San Francisco"},
			//StreetAddress: []string{"Golden Gate Bridge"},
			//PostalCode:    []string{"94016"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	pk, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	bs, err := x509.CreateCertificate(rand.Reader, ca, ca, &pk.PublicKey, pk)
	if err != nil {
		return err
	}

	f, err := os.Create(cafile)
	if err != nil {
		return err
	}
	pem.Encode(f, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: bs,
	})
	//f.Close()

	// f, err = os.Create(capkfile)
	// if err != nil {
	// 	return err
	// }
	pem.Encode(f, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pk),
	})
	f.Close()

	return nil
}

// GenCert generates a certificate for a user with name `uname` and writes it to `certfile`.
// This cert is signed by the certificate authority `ca` and its private key `caPrivkey`.
func GenCert(uname, certfile string, ca *x509.Certificate, caPrivkey *rsa.PrivateKey) error {
	ctemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization: []string{"go9p"},
			//Country:       []string{"US"},
			//Province:      []string{""},
			//Locality:      []string{"San Francisco"},
			//StreetAddress: []string{"Golden Gate Bridge"},
			//PostalCode:    []string{"94016"},
			CommonName: uname,
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPK, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	bs, err := x509.CreateCertificate(rand.Reader, ctemplate, ca, &certPK.PublicKey, caPrivkey)
	if err != nil {
		return err
	}

	f, err := os.Create(certfile)
	if err != nil {
		return err
	}
	pem.Encode(f, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: bs,
	})

	pem.Encode(f, &pem.Block{
		Type:  "CA",
		Bytes: ca.Raw,
	})

	pem.Encode(f, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPK),
	})
	f.Close()

	return nil
}

// LoadCA loads a certificate and its private key out of certf
func LoadCA(certf string) (ca *x509.Certificate, pk *rsa.PrivateKey, err error) {
	f, err := os.Open(certf)
	if err != nil {
		return nil, nil, err
	}
	cabs, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	f.Close()

	//var cert *x509.Certificate
	//var pk *rsa.PrivateKey

	for len(cabs) > 0 {
		block, rem := pem.Decode(cabs)
		if block == nil {
			return nil, nil, fmt.Errorf("Failed to decode %s. Is it in PEM format?", certf)
		}
		if block.Type == "CERTIFICATE" {
			ca, err = x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, err
			}
		} else if block.Type == "RSA PRIVATE KEY" {
			pk, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, nil, err
			}
		} else {
			return nil, nil, fmt.Errorf("Failed to decode %s. Found unknown section %s", certf, block.Type)
		}
		cabs = rem
	}
	if ca == nil {
		return nil, nil, fmt.Errorf("Failed to decode %s. Could not find section 'CERTIFICATE'", certf)
	}
	if pk == nil {
		return nil, nil, fmt.Errorf("Failed to decode %s. Could not find section 'RSA PRIVATE KEY'", certf)
	}
	return ca, pk, nil
}

// LoadCert loads a certificate and its private key out of certf
func LoadCert(certf string) (cert *x509.Certificate, pk *rsa.PrivateKey, ca *x509.Certificate, err error) {
	f, err := os.Open(certf)
	if err != nil {
		return nil, nil, nil, err
	}
	cabs, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, nil, err
	}
	f.Close()

	//var cert *x509.Certificate
	//var pk *rsa.PrivateKey

	for len(cabs) > 0 {
		block, rem := pem.Decode(cabs)
		if block == nil {
			return nil, nil, nil, fmt.Errorf("Failed to decode %s. Is it in PEM format?", certf)
		}
		if block.Type == "CERTIFICATE" {
			cert, err = x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, nil, err
			}
		} else if block.Type == "RSA PRIVATE KEY" {
			pk, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, nil, nil, err
			}
		} else if block.Type == "CA" {
			ca, err = x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, nil, err
			}
		} else {
			return nil, nil, nil, fmt.Errorf("Failed to decode %s. Found unknown section %s", certf, block.Type)
		}
		cabs = rem
	}
	if cert == nil {
		return nil, nil, nil, fmt.Errorf("Failed to decode %s. Could not find section 'CERTIFICATE'", certf)
	}
	if pk == nil {
		return nil, nil, nil, fmt.Errorf("Failed to decode %s. Could not find section 'RSA PRIVATE KEY'", certf)
	}
	return cert, pk, ca, nil
}

// LoadCert loads a certificate and its private key out of certf and returns them
// as a tls.Certificate
func LoadTLSCert(certf string) (cert tls.Certificate, ca *x509.Certificate, err error) {
	f, err := os.Open(certf)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	certfbs, err := io.ReadAll(f)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	f.Close()

	var certbs []byte
	var pkbs []byte
	var cabs []byte

	for len(certfbs) > 0 {
		block, rem := pem.Decode(certfbs)
		if block == nil {
			return tls.Certificate{}, nil, fmt.Errorf("Failed to decode %s. Is it in PEM format?", certf)
		}
		fmt.Printf("FOUND BLOCK TYPE %s\n", block.Type)
		if block.Type == "CERTIFICATE" {
			certbs = block.Bytes
		} else if block.Type == "RSA PRIVATE KEY" {
			pkbs = block.Bytes
		} else if block.Type == "CA" {
			cabs = block.Bytes
		} else {
			return tls.Certificate{}, nil, fmt.Errorf("Failed to decode %s. Found unknown section %s", certf, block.Type)
		}
		certfbs = rem
	}
	if certbs == nil {
		return tls.Certificate{}, nil, fmt.Errorf("Failed to decode %s. Could not find section 'CERTIFICATE'", certf)
	}
	if pkbs == nil {
		return tls.Certificate{}, nil, fmt.Errorf("Failed to decode %s. Could not find section 'RSA PRIVATE KEY'", certf)
	}
	if cabs == nil {
		return tls.Certificate{}, nil, fmt.Errorf("Failed to decode %s. Could not find section 'CA'", certf)
	}

	xc, err := x509.ParseCertificate(certbs)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("Could not load certificate: %v", err)
	}
	cert.Certificate = append(cert.Certificate, xc.Raw)
	cert.PrivateKey, err = x509.ParsePKCS1PrivateKey(pkbs)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("Could not load certificate: %v", err)
	}
	// cert, err = tls.X509KeyPair(certbs, pkbs)
	// if err != nil {
	// 	return tls.Certificate{}, nil, fmt.Errorf("Could not load certificate: %v", err)
	// }

	ca, err = x509.ParseCertificate(cabs)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("Could not load cert authority: %v", err)
	}

	return cert, ca, nil
}

func TLSCertUser(c *tls.Certificate) (string, error) {
	xc, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		return "", fmt.Errorf("Failed to parse x509 from tls cert: %v", err)
	}

	return CertUser(xc), nil
}

func CertUser(c *x509.Certificate) string {
	return c.Subject.CommonName
}
