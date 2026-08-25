package store

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

const maxFrameSize = 32 << 20

func frameChecksum(frame EventFrame) (string, error) {
	frame.Checksum = ""
	data, err := json.Marshal(frame)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func appendFrame(file *os.File, frame EventFrame) error {
	checksum, err := frameChecksum(frame)
	if err != nil {
		return err
	}
	frame.Checksum = checksum
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if len(data) > maxFrameSize {
		return fmt.Errorf("event frame too large: %d", len(data))
	}
	header := make([]byte, 8)
	binary.BigEndian.PutUint64(header, uint64(len(data)))
	if _, err := file.Write(header); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func readFrames(file *os.File) ([]EventFrame, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	frames := make([]EventFrame, 0)
	previous := ""
	expected := uint64(1)
	for {
		header := make([]byte, 8)
		_, err := io.ReadFull(reader, header)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: 事件帧长度被截断", domain.ErrIntegrity)
		}
		size := binary.BigEndian.Uint64(header)
		if size == 0 || size > maxFrameSize {
			return nil, fmt.Errorf("%w: 非法事件帧长度", domain.ErrIntegrity)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, fmt.Errorf("%w: 事件帧内容被截断", domain.ErrIntegrity)
		}
		var frame EventFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, fmt.Errorf("%w: 无法解析事件帧", domain.ErrIntegrity)
		}
		if frame.SchemaVersion != schemaVersion || frame.Sequence != expected || frame.PreviousHash != previous {
			return nil, fmt.Errorf("%w: 事件链序号或前序摘要错误", domain.ErrIntegrity)
		}
		checksum, err := frameChecksum(frame)
		if err != nil || checksum != frame.Checksum {
			return nil, fmt.Errorf("%w: 事件帧校验和错误", domain.ErrIntegrity)
		}
		frames = append(frames, frame)
		previous = frame.Checksum
		expected++
	}
	return frames, nil
}
