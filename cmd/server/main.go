package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/httpui"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "服务退出:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	configuration, err := parseConfig(arguments)
	if err != nil {
		return err
	}
	dataDirectory := configuration.dataDirectory
	if configuration.selfcheck {
		dataDirectory, err = os.MkdirTemp("", "oral-history-selfcheck-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dataDirectory)
	}
	repository, err := store.Open(dataDirectory)
	if err != nil {
		return fmt.Errorf("打开本地存储: %w", err)
	}
	defer repository.Close()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := application.NewService(repository)
	ui := httpui.New(service, logger)
	listener, err := net.Listen("tcp", configuration.address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", configuration.address, err)
	}
	server := &http.Server{Handler: ui.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	logger.Info("口述史公开授权工作台已启动", "addr", listener.Addr().String(), "selfcheck", configuration.selfcheck)
	if configuration.selfcheck {
		checkErr := runSelfcheck("http://" + listener.Addr().String())
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownContext)
		serveErr := <-serveResult
		if checkErr != nil {
			return fmt.Errorf("自检失败: %w", checkErr)
		}
		if shutdownErr != nil {
			return fmt.Errorf("自检关闭失败: %w", shutdownErr)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		integrity, integrityErr := repository.InspectIntegrity()
		if integrityErr != nil || !integrity.Valid {
			return fmt.Errorf("自检持久化完整性失败: %w", integrityErr)
		}
		logger.Info("持久化事件链与原子快照一致", "events", integrity.EventFrameCount, "sequence", integrity.LastSequence)
		logger.Info("完整 HTTP 业务自检通过")
		return nil
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case signalValue := <-stop:
		logger.Info("收到关闭信号", "signal", signalValue.String())
	case serveErr := <-serveResult:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return err
	}
	serveErr := <-serveResult
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}
