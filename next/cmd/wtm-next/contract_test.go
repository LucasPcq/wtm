package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEachMutationKeepsTheCatalogueContract(t *testing.T) {
	for _, m := range mutations() {
		t.Run(m.name, func(t *testing.T) { keepsTheMutationContract(t, m) })
	}
}

func TestEachReadKeepsTheCatalogueContract(t *testing.T) {
	for _, name := range reads {
		t.Run(name, func(t *testing.T) { keepsTheReadContract(t, name) })
	}
}

func TestEveryCommandOfTheCatalogueIsUnderItsContract(t *testing.T) {
	covered := append([]string{}, reads...)
	for _, m := range mutations() {
		covered = append(covered, m.name)
	}
	var listed []string
	for _, command := range catalogue() {
		listed = append(listed, command.Name)
	}

	assert.ElementsMatch(t, listed, covered, "a command of the catalogue is neither a mutation nor a read of the contract")
}
