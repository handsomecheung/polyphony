package markdown

import (
	"koishi/chameleon/homego/internal/common"

	"github.com/russross/blackfriday/v2"
)

func Content2HTML(content string) string {
	return Bytes2HTML([]byte(content))
}

func File2HTML(filename string) string {
	return Bytes2HTML(common.ReadFile(filename))
}

func Bytes2HTML(bytes []byte) string {
	return string(blackfriday.Run(bytes))
}

func GetFileContent(filename string) string {
	return string(common.ReadFile(filename))
}
