package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type config struct {
	address       string
	dataDirectory string
	selfcheck     bool
}

func defaultAddress() (string, error) {
	value := strings.TrimSpace(os.Getenv("PORT"))
	if value == "" {
		return "127.0.0.1:19081", nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("PORT 必须是 1 到 65535 之间的端口号")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

func parseConfig(arguments []string) (config, error) {
	address, err := defaultAddress()
	if err != nil {
		return config{}, err
	}
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	value := config{}
	flags.StringVar(&value.address, "addr", address, "HTTP 监听地址，仅允许回环地址")
	flags.StringVar(&value.dataDirectory, "data", "./data", "本地持久化目录")
	flags.BoolVar(&value.selfcheck, "selfcheck", false, "启动真实 HTTP 服务并执行完整业务冒烟后退出")
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("不支持的位置参数: %s", strings.Join(flags.Args(), " "))
	}
	if err := validateAddress(value.address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(value.dataDirectory) == "" {
		return config{}, errors.New("-data 不能为空")
	}
	return value, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 必须是 host:port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("-addr 端口必须在 1 到 65535 之间")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("-addr 仅允许 127.0.0.0/8、::1 或 localhost 回环地址")
	}
	return nil
}
