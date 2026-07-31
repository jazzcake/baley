package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jazzcake/baley/server/internal/projectinit"
)

func main() {
	projectRoot := flag.String("project-root", ".", "existing Git project root")
	inputPath := flag.String("input", "-", "bootstrap input JSON path, or - for stdin")
	apply := flag.Bool("apply", false, "apply only verified create/merge actions")
	flag.Parse()

	var source io.Reader
	var err error
	if *inputPath == "-" {
		source = os.Stdin
	} else {
		var inputFile *os.File
		inputFile, err = os.Open(*inputPath)
		if err != nil {
			fatal(err)
		}
		defer inputFile.Close()
		source = inputFile
	}
	encodedInput, err := io.ReadAll(source)
	if err != nil {
		fatal(err)
	}
	encodedInput = bytes.TrimPrefix(encodedInput, []byte{0xef, 0xbb, 0xbf})
	var input projectinit.Input
	decoder := json.NewDecoder(bytes.NewReader(encodedInput))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		fatal(err)
	}
	input.ExistingFiles, err = projectinit.LoadExistingFiles(*projectRoot, input.TaskRecordsRoot)
	if err != nil {
		fatal(err)
	}
	plan, err := projectinit.Build(input)
	if err != nil {
		fatal(err)
	}
	if *apply {
		if !input.BootstrapCompleted {
			fatal(fmt.Errorf("server Workspace/repository binding must be completed before apply"))
		}
		if err = projectinit.Apply(*projectRoot, plan); err != nil {
			fatal(err)
		}
	}
	encoded, _ := json.MarshalIndent(plan, "", "  ")
	fmt.Println(string(encoded))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "baley-project-init:", err)
	os.Exit(1)
}
