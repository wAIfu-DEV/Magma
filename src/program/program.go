package program

import (
	"Magma/src/types"
	"errors"
	"fmt"
)

func JoinCompilationUnits(shared *types.SharedState, e error) error {
	shared.WaitGroup.Wait()

	e2 := e

	for k, v := range shared.ImportedFiles {
		err := <-v
		if err != nil {
			e2 = errors.Join(e2, err)
			fmt.Printf("fatal error in file '%s': %s\n", k, err.Error())
		}
	}
	return e2
}
