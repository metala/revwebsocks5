package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	keyOut      string
	certOut     string
	dnsNames    []string
	ipAddresses []net.IP
)

// keygenCmd represents the keygen command
var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a TLS key and certificate",
	Long:  `The utility creates the required TLS key and certificate for the server (and client).`,
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := os.Stat(keyOut); err == nil {
			log.Fatalf("key file '%s' exists", keyOut)
		}
		if _, err := os.Stat(certOut); err == nil {
			log.Fatalf("cert file '%s' exists", certOut)
		}

		key, cert := genKeyCert()
		certPem, keyPem := getPEMs(cert, key)
		if err := os.WriteFile(keyOut, keyPem, 0o0600); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote key to: %s", keyOut)
		if err := os.WriteFile(certOut, certPem, 0o0644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote certificate to: %s", certOut)
	},
}

func init() {
	rootCmd.AddCommand(keygenCmd)

	keygenCmd.Flags().StringVarP(&keyOut, "key-out", "k", "./tls/server.key", "the key output filename")
	keygenCmd.Flags().StringVarP(&certOut, "cert-out", "c", "./tls/server.crt", "the certificate output filename")
	keygenCmd.Flags().StringSliceVarP(&dnsNames, "dns-name", "D", []string{"localhost"}, "add dns name")
	keygenCmd.Flags().IPSliceVarP(&ipAddresses, "ip-addr", "I", []net.IP{net.IPv4(127, 0, 0, 1)}, "add ip address")
}

func getPEMs(cert []byte, key []byte) (pemcert []byte, pemkey []byte) {
	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})

	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: key,
	})

	return certPem, keyPem
}

func genKeyCert() (key []byte, cert []byte) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	ca := &x509.Certificate{
		SerialNumber: RandBigInt(serialNumberLimit),
		Subject: pkix.Name{
			Country:            []string{RandString(16)},
			Organization:       []string{RandString(16)},
			OrganizationalUnit: []string{RandString(16)},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		SubjectKeyId:          RandBytes(5),
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,

		IPAddresses: ipAddresses,
		DNSNames:    dnsNames,
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Println("generate key", err)
		return
	}
	certDer, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		log.Println("self sign", err)
		return
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Println("marshal key", err)
		return
	}

	return privBytes, certDer

}

// RandBytes generates random bytes of n size
// It returns the generated random bytes
func RandBytes(n int) []byte {
	r := make([]byte, n)
	_, _ = rand.Read(r)
	return r
}

// RandBigInt generates random big integer with max number
// It returns the generated random big integer
func RandBigInt(max *big.Int) *big.Int {
	r, _ := rand.Int(rand.Reader, max)
	return r
}
