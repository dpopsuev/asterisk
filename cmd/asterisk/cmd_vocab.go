package main

import (
	framework "github.com/dpopsuev/origami"

	"asterisk/adapters/vocabulary"
)

var cmdVocab = vocabulary.New()

func vocabName(code string) string {
	return cmdVocab.Name(code)
}

func vocabNameWithCode(code string) string {
	return framework.NameWithCode(cmdVocab, code)
}
