package main

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"

	"github.com/crunk1/go-jsonschema/2019-09"
)

func main() {
	s, err := jsonschema.FromURI("https://raw.githubusercontent.com/OAI/OpenAPI-Specification/master/schemas/v3.0/schema.json")
	fmt.Println(err)
	fmt.Println(s.AdditionalProperties.AsBool())
	spew.Dump(s)
}
