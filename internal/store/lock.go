package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquireLock 对目录内的锁文件加排他非阻塞 flock，确保同一存储目录同一时刻
// 只能被一个存活的实例写入。进程退出（包括崩溃）时内核自动释放锁，因此
// 关闭或重启后仍可正常重新打开。
func acquireLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("存储目录 %s 已被另一个实例占用", filepath.Dir(path))
		}
		return nil, err
	}
	return file, nil
}
