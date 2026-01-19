package common

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/handsomecheung/mb64"
)

func ReadFile(filename string) []byte {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil
	}
	return content
}

func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !errors.Is(err, os.ErrNotExist)
}

func IsDigital(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func Bytes2MB(bs []byte) string {
	encoded, err := mb64.Encode(bs)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func String2MB(s string) string {
	return Bytes2MB([]byte(s))
}

func File2MB(filepath string) string {
	return Bytes2MB(ReadFile(filepath))
}

func RandFloatRange(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

func RandIntRange(value int, ratio float64) int {
	return int(math.Ceil(float64(value) * RandFloatRange(1-ratio, 1+ratio)))
}

func CallLowkey(messageFilePath, sourceImagePath, targetImagePath string) {
	err1 := os.MkdirAll(filepath.Dir(targetImagePath), 0755)
	if err1 != nil {
		fmt.Printf("Error running command mkdir: %v\n", err1)
	}

	cmd := exec.Command("lowkey", "encode", "--image", sourceImagePath, "--message", messageFilePath, "--output", targetImagePath, "--auto-resize")
	output, err2 := cmd.Output()
	if err2 != nil {
		fmt.Printf("Error running command lowkey: %v\n", err2)
	}
	fmt.Printf("Command lowkey output:\n%s\n", output)
}
