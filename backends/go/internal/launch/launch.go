package launch

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type TLSFiles struct {
	Cert string
	Key  string
}

type ListenFunc func(network, address string) (net.Listener, error)

func ConfigDir(override string) (string, error) {
	if override != "" {
		return filepath.Clean(override), nil
	}
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find the user configuration directory: %w", err)
	}
	return filepath.Join(userConfigDir, "bigbrother"), nil
}

func TLSFilesForConfigDir(configDir, certOverride, keyOverride string) (TLSFiles, error) {
	if (certOverride == "") != (keyOverride == "") {
		return TLSFiles{}, errors.New("--tls-cert and --tls-key must be supplied together")
	}
	if certOverride != "" {
		return TLSFiles{Cert: certOverride, Key: keyOverride}, nil
	}
	return TLSFiles{
		Cert: filepath.Join(configDir, "tls", "server.crt"),
		Key:  filepath.Join(configDir, "tls", "server.key"),
	}, nil
}

func LoadTLSCertificate(files TLSFiles) (tls.Certificate, error) {
	certificate, err := tls.LoadX509KeyPair(files.Cert, files.Key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load TLS certificate %q and key %q: %w; run scripts/setup-tls or provide both --tls-cert and --tls-key", files.Cert, files.Key, err)
	}
	return certificate, nil
}

func DefaultPorts() []int {
	ports := []int{443, 8443}
	for port := 8444; port <= 8453; port++ {
		ports = append(ports, port)
	}
	return append(ports, 0)
}

func SelectListener(explicitPort int, candidates []int, listen ListenFunc) (net.Listener, int, error) {
	if listen == nil {
		listen = net.Listen
	}
	if explicitPort != 0 {
		listener, err := listen("tcp", net.JoinHostPort("", strconv.Itoa(explicitPort)))
		if err != nil {
			return nil, 0, fmt.Errorf("listen on explicit port %d: %w", explicitPort, err)
		}
		return listener, listenerPort(listener), nil
	}
	if candidates == nil {
		candidates = DefaultPorts()
	}
	for index, port := range candidates {
		listener, err := listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
		if err == nil {
			return listener, listenerPort(listener), nil
		}
		if !canTryNextPort(err) || index == len(candidates)-1 {
			return nil, 0, fmt.Errorf("listen on port %d: %w", port, err)
		}
	}
	return nil, 0, errors.New("no candidate ports were configured")
}

func listenerPort(listener net.Listener) int {
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0
	}
	return address.Port
}

func canTryNextPort(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EADDRINUSE)
}

func LANURLs(port int) ([]string, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("list local network addresses: %w", err)
	}
	seen := make(map[string]struct{})
	var urls []string
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || !ip.IsGlobalUnicast() {
			continue
		}
		host := ip.String()
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		url := "https://" + host
		if port != 443 {
			url += ":" + strconv.Itoa(port)
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
	}
	sort.Strings(urls)
	if len(urls) == 0 {
		return []string{"https://localhost" + portSuffix(port)}, nil
	}
	return urls, nil
}

func portSuffix(port int) string {
	if port == 443 {
		return ""
	}
	return ":" + strconv.Itoa(port)
}

func StaticDir(executablePath, workingDir string) string {
	if executablePath != "" {
		candidate := filepath.Join(filepath.Dir(executablePath), "web")
		if isDirectory(candidate) {
			return candidate
		}
	}
	for _, candidate := range []string{
		filepath.Join(workingDir, "web", "dist"),
		filepath.Join(workingDir, "..", "..", "web", "dist"),
	} {
		if isDirectory(candidate) {
			return candidate
		}
	}
	return filepath.Join(workingDir, "web", "dist")
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
