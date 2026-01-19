//go:build js && wasm
// +build js,wasm

package main

import (
	"fmt"
	"syscall/js"

	"koishi/chameleon/homego/internal/markdown"
	"koishi/chameleon/homego/internal/tag"

	"github.com/handsomecheung/mb64"
)

func main() {
	RegisterWasmFunctions()

	select {}
}

func wrapMethod(method func(args []js.Value) interface{}) func(this js.Value, args []js.Value) interface{} {
	return func(this js.Value, args []js.Value) interface{} {
		var value interface{}
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					value = js.ValueOf("error: " + err.Error())
				} else {
					value = js.ValueOf("error: " + fmt.Sprint(r))
				}
			}
		}()
		value = method(args)
		return value
	}
}

func click(args []js.Value) interface{} {
	if len(args) != 1 {
		return js.ValueOf("error: expected 1 argument")
	}

	input := args[0].String()
	response := tag.Click(input)
	return js.ValueOf(response)
}

func renderMBMarkdown(args []js.Value) interface{} {
	if len(args) != 1 {
		return js.ValueOf("error: expected 1 argument")
	}

	input := args[0].String()
	bytes, err := mb64.Decode([]byte(input))
	if err != nil {
		panic(err)
	}

	return js.ValueOf(markdown.Bytes2HTML(bytes))
}

func setFont(args []js.Value) interface{} {
	if len(args) != 1 {
		return js.ValueOf("error: expected 1 argument")
	}

	input := args[0].String()
	err := mb64.SetEncoding(input)
	if err != nil {
		panic(err)
	}

	return nil
}

func RegisterWasmFunctions() {
	js.Global().Set("main", map[string]interface{}{
		"click":            js.FuncOf(wrapMethod(click)),
		"setFont":          js.FuncOf(wrapMethod(setFont)),
		"renderMBMarkdown": js.FuncOf(wrapMethod(renderMBMarkdown)),
	})
}
