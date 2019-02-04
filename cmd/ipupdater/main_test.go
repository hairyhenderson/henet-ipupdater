package main

import (
	"testing"

	"github.com/spf13/cobra/doc"
)

func TestMakeMan(t *testing.T) {
	header := &doc.GenManHeader{
		Title:   "IPUPDATER",
		Section: "3",
	}
	cmd := newCmd()
	initFlags(cmd)
	err := doc.GenManTree(cmd, header, "/tmp")
	if err != nil {
		t.Fatal(err)
	}

	doc.GenYamlTree(cmd, "/tmp")
}
