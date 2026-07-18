package join

import (
	"Magma/src/comp_err"
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
			// The main compilation unit publishes the same error returned by
			// pipeline.DoMain. It has already been reported by the caller.
			if err == e {
				continue
			}
			e2 = errors.Join(e2, err)
			if !comp_err.Print(err) {
				fmt.Printf("fatal error in file '%s': %s\n", k, err.Error())
			}
		}
	}
	return e2
}
