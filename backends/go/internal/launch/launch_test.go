package launch

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExplicitPortFailureDoesNotFallBack(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	listener, selectedPort, err := SelectListener(port, nil, net.Listen)
	if listener != nil {
		listener.Close()
		t.Fatal("SelectListener returned a listener for an occupied explicit port")
	}
	if selectedPort != 0 {
		t.Fatalf("selected port = %d, want 0 on failure", selectedPort)
	}
	if err == nil || !strings.Contains(err.Error(), "explicit port") {
		t.Fatalf("error = %v, want an explicit-port failure", err)
	}
}

func TestDefaultPortSelectionFallsBackOnPermissionAndAddressInUse(t *testing.T) {
	for _, initialError := range []error{syscall.EACCES, syscall.EADDRINUSE} {
		t.Run(initialError.Error(), func(t *testing.T) {
			var tried []int
			listen := func(network, address string) (net.Listener, error) {
				_, portText, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				port, err := strconv.Atoi(portText)
				if err != nil {
					return nil, err
				}
				tried = append(tried, port)
				if len(tried) == 1 {
					return nil, &net.OpError{Op: "listen", Net: network, Err: initialError}
				}
				return net.Listen(network, "127.0.0.1:0")
			}

			listener, selectedPort, err := SelectListener(0, []int{443, 8443, 0}, listen)
			if err != nil {
				t.Fatalf("SelectListener: %v", err)
			}
			defer listener.Close()
			if len(tried) != 2 || tried[0] != 443 || tried[1] != 8443 {
				t.Fatalf("attempted ports = %v, want [443 8443]", tried)
			}
			if selectedPort == 0 {
				t.Fatal("fallback listener has no selected port")
			}
		})
	}
}

func TestDefaultPortSelectionStopsOnUnrelatedFailure(t *testing.T) {
	want := errors.New("bad network configuration")
	calls := 0
	listen := func(string, string) (net.Listener, error) {
		calls++
		return nil, want
	}

	_, _, err := SelectListener(0, []int{443, 8443}, listen)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want wrapped configuration failure", err)
	}
	if calls != 1 {
		t.Fatalf("listen calls = %d, want 1", calls)
	}
}

func TestDefaultConfigDirAndOverride(t *testing.T) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ConfigDir("")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(userConfigDir, "bigbrother"); got != want {
		t.Fatalf("default config dir = %q, want %q", got, want)
	}

	override := filepath.Join(t.TempDir(), "private-config")
	got, err = ConfigDir(override)
	if err != nil {
		t.Fatal(err)
	}
	if got != override {
		t.Fatalf("override config dir = %q, want %q", got, override)
	}
}

func TestTLSOverridesMustBePaired(t *testing.T) {
	if _, err := TLSFilesForConfigDir(t.TempDir(), "custom.crt", ""); err == nil {
		t.Fatal("TLSFilesForConfigDir accepted a certificate override without a key override")
	}
	if _, err := TLSFilesForConfigDir(t.TempDir(), "", "custom.key"); err == nil {
		t.Fatal("TLSFilesForConfigDir accepted a key override without a certificate override")
	}
}

func TestTLSPathsUseConfigDirectoryOrPairedOverrides(t *testing.T) {
	configDir := t.TempDir()
	files, err := TLSFilesForConfigDir(configDir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if files.Cert != filepath.Join(configDir, "tls", "server.crt") || files.Key != filepath.Join(configDir, "tls", "server.key") {
		t.Fatalf("default TLS files = %#v", files)
	}

	files, err = TLSFilesForConfigDir(configDir, "/certs/local.crt", "/keys/local.key")
	if err != nil {
		t.Fatal(err)
	}
	if files.Cert != "/certs/local.crt" || files.Key != "/keys/local.key" {
		t.Fatalf("override TLS files = %#v", files)
	}
}

func TestLoadTLSCertificateReportsMissingAndInvalidFiles(t *testing.T) {
	missing := TLSFiles{Cert: filepath.Join(t.TempDir(), "missing.crt"), Key: filepath.Join(t.TempDir(), "missing.key")}
	if _, err := LoadTLSCertificate(missing); err == nil || !strings.Contains(err.Error(), "TLS certificate") {
		t.Fatalf("missing-file error = %v, want actionable TLS certificate error", err)
	}

	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	if err := os.WriteFile(certPath, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTLSCertificate(TLSFiles{Cert: certPath, Key: keyPath}); err == nil {
		t.Fatal("LoadTLSCertificate accepted invalid certificate files")
	}
}

func TestDefaultPortsStartWith443AndEndWithEphemeralPort(t *testing.T) {
	ports := DefaultPorts()
	if len(ports) != 13 || ports[0] != 443 || ports[1] != 8443 || ports[len(ports)-1] != 0 {
		t.Fatalf("DefaultPorts() = %v", ports)
	}
}

func TestStaticDirFindsPackagedAssetsAndDevelopmentAssets(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "package")
	if err := os.MkdirAll(filepath.Join(packageDir, "web"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, want := StaticDir(filepath.Join(packageDir, "bigbrother"), root), filepath.Join(packageDir, "web"); got != want {
		t.Fatalf("packaged static dir = %q, want %q", got, want)
	}

	repositoryDir := filepath.Join(root, "repo")
	backendDir := filepath.Join(repositoryDir, "backends", "go")
	devAssets := filepath.Join(repositoryDir, "web", "dist")
	if err := os.MkdirAll(backendDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(devAssets, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := StaticDir(filepath.Join(root, "go-build"), backendDir); got != devAssets {
		t.Fatalf("development static dir = %q, want %q", got, devAssets)
	}
}

func TestLoadTLSCertificateReturnsCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := writeTestCertificate(certPath, keyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTLSCertificate(TLSFiles{Cert: certPath, Key: keyPath}); err != nil {
		t.Fatalf("LoadTLSCertificate: %v", err)
	}
}

func writeTestCertificate(certPath, keyPath string) error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0o600); err != nil {
		return err
	}
	return os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600)
}
