package join

import (
	"Magma/src/comp_err"
	"Magma/src/types"
	"sort"
)

func JoinCompilationUnits(shared *types.SharedState, e error) error {
	shared.WaitGroup.Wait()

	errs := []error{e}
	paths := make([]string, 0, len(shared.ImportedFiles))
	for path := range shared.ImportedFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		v := shared.ImportedFiles[path]
		err := <-v
		if err != nil {
			// The main compilation unit publishes the same error returned by
			// pipeline.DoMain. It has already been reported by the caller.
			if err == e {
				continue
			}
			errs = append(errs, err)
		}
	}
	return comp_err.Join(errs...)
}
