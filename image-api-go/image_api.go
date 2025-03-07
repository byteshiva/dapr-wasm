package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/second-state/WasmEdge-go/wasmedge"

	"github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/http"
)

func imageHandlerWASI(_ context.Context, in *common.InvocationEvent) (out *common.Content, err error) {
	image := in.Data

	/// Set not to print debug info
	wasmedge.SetLogErrorLevel()

	/// Create configure
	var conf = wasmedge.NewConfigure(wasmedge.REFERENCE_TYPES)
	conf.AddConfig(wasmedge.WASI)

	/// Create VM with configure
	var vm = wasmedge.NewVMWithConfig(conf)

	/// Init WASI (test)
	var wasi = vm.GetImportObject(wasmedge.WASI)
	wasi.InitWasi(
		os.Args[1:],     /// The args
		os.Environ(),    /// The envs
		[]string{".:."}, /// The mapping directories
		[]string{},      /// The preopens will be empty
	)

	/// Register WasmEdge-tensorflow and WasmEdge-image
	var tfobj = wasmedge.NewTensorflowImportObject()
	var tfliteobj = wasmedge.NewTensorflowLiteImportObject()
	vm.RegisterImport(tfobj)
	vm.RegisterImport(tfliteobj)
	var imgobj = wasmedge.NewImageImportObject()
	vm.RegisterImport(imgobj)
	/// Instantiate wasm

	vm.LoadWasmFile("./lib/classify_bg.wasm")
	vm.Validate()
	vm.Instantiate()

	res, err := vm.ExecuteBindgen("infer", wasmedge.Bindgen_return_array, image)
	ans := string(res.([]byte))
	if err != nil {
		println("error: ", err.Error())
	}

	vm.Delete()
	conf.Delete()

	fmt.Printf("Image classify result: %q\n", ans)
	out = &common.Content{
		Data:        []byte(ans),
		ContentType: in.ContentType,
		DataTypeURL: in.DataTypeURL,
	}
	return out, nil
}

// currently don't use it, only for demo
func imageHandler(_ context.Context, in *common.InvocationEvent) (out *common.Content, err error) {
	image := string(in.Data)
	cmd := exec.Command("./lib/wasmedge-tensorflow-lite", "./lib/classify.so")
	cmd.Stdin = strings.NewReader(image)

	var std_out bytes.Buffer
	cmd.Stdout = &std_out
	cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	res := std_out.String()
	fmt.Printf("Image classify result: %q\n", res)
	out = &common.Content{
		Data:        []byte(res),
		ContentType: in.ContentType,
		DataTypeURL: in.DataTypeURL,
	}
	return out, nil
}

func main() {
	s := daprd.NewService(":9003")

	if err := s.AddServiceInvocationHandler("/api/image", imageHandlerWASI); err != nil {
		log.Fatalf("error adding invocation handler: %v", err)
	}

	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error listenning: %v", err)
	}
}
