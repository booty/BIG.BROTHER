package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bigbrother/internal/launch"
	"bigbrother/internal/media"
	"bigbrother/internal/room"
	"bigbrother/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "BIG.BROTHER:", err)
		os.Exit(1)
	}
}

func run() error {
	port := flag.Int("port", 0, "HTTPS port; when omitted, try 443 and fallback ports")
	configDirFlag := flag.String("config-dir", "", "configuration directory")
	tlsCertFlag := flag.String("tls-cert", "", "TLS certificate file (requires --tls-key)")
	tlsKeyFlag := flag.String("tls-key", "", "TLS private key file (requires --tls-cert)")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flag.Args())
	}
	if *port < 0 || *port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	configDir, err := launch.ConfigDir(*configDirFlag)
	if err != nil {
		return err
	}
	tlsFiles, err := launch.TLSFilesForConfigDir(configDir, *tlsCertFlag, *tlsKeyFlag)
	if err != nil {
		return err
	}
	certificate, err := launch.LoadTLSCertificate(tlsFiles)
	if err != nil {
		return err
	}
	executablePath, _ := os.Executable()
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("find the current directory: %w", err)
	}
	mediaBinary, mediaConfig, err := media.ResolvePaths(executablePath, workingDir)
	if err != nil {
		return err
	}
	mediaManager := media.New(media.Options{
		BinaryPath:    mediaBinary,
		ConfigPath:    mediaConfig,
		HTTPURL:       "http://127.0.0.1:8889",
		ControlAPIURL: "http://127.0.0.1:9997",
		Logger:        nil,
	})
	mediaStartContext, cancelMediaStart := context.WithTimeout(context.Background(), 15*time.Second)
	err = mediaManager.Start(mediaStartContext)
	cancelMediaStart()
	if err != nil {
		return err
	}
	defer func() {
		if err := mediaManager.Stop(); err != nil {
			fmt.Fprintln(os.Stderr, "BIG.BROTHER: stop MediaMTX:", err)
		}
	}()
	mediaProxy, err := media.NewProxy(mediaManager, "http://127.0.0.1:8889")
	if err != nil {
		return err
	}
	listener, selectedPort, err := launch.SelectListener(*port, launch.DefaultPorts(), nil)
	if err != nil {
		return err
	}

	staticDir := launch.StaticDir(executablePath, workingDir)
	roomState := room.New(room.Options{
		Logger:        slog.New(slog.NewTextHandler(os.Stdout, nil)),
		AssignStream:  mediaManager.AssignStream,
		ReleaseStream: mediaManager.ReleaseStream,
	})
	handler := server.NewHandler(staticDir, server.RoomWebSocket(roomState), mediaProxy, mediaManager.StatusHandler())
	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	tlsListener := tls.NewListener(listener, &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	})

	urls, err := launch.LANURLs(selectedPort)
	if err != nil {
		listener.Close()
		return err
	}
	fmt.Fprintln(os.Stdout, "BIG.BROTHER listening at:")
	for _, url := range urls {
		fmt.Fprintln(os.Stdout, " ", url)
	}

	serveErrors := make(chan error, 1)
	go func() { serveErrors <- httpServer.Serve(tlsListener) }()
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTPS: %w", err)
		}
		return nil
	case <-shutdownContext.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shut down HTTPS server: %w", err)
		}
		err := <-serveErrors
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTPS: %w", err)
		}
		return nil
	}
}
