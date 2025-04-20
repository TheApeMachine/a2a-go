package utils

import (
	"bufio"
	"errors"
	"strings"
)

func ReadSSE(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')

	if err != nil {
		return "", err
	}

	line = strings.TrimSpace(line)

	if line == "" || strings.HasPrefix(line, ":") { // comments / keep‑alive
		return "", nil
	}

	if !strings.HasPrefix(line, "data: ") {
		return "", errors.New("invalid SSE line")
	}

	data := strings.TrimPrefix(line, "data: ")

	return data, nil
}
