package loweringbackend

import "fmt"

type blockState struct {
	function   FunctionID
	name       string
	terminated bool
}

// CFG tracks block ownership and completion independently of LLVM. Backends
// use it before appending instructions so malformed control flow cannot reach
// a verifier or native-library assertion.
type CFG struct {
	blocks       []blockState
	byFunction   map[FunctionID][]BlockID
	predecessors map[BlockID]map[BlockID]struct{}
}

func NewCFG() *CFG {
	return &CFG{byFunction: make(map[FunctionID][]BlockID), predecessors: make(map[BlockID]map[BlockID]struct{})}
}

func (c *CFG) AppendBlock(function FunctionID, name string) (BlockID, error) {
	if function == 0 {
		return 0, fmt.Errorf("block has no function owner")
	}
	id := BlockID(len(c.blocks) + 1)
	c.blocks = append(c.blocks, blockState{function: function, name: name})
	c.byFunction[function] = append(c.byFunction[function], id)
	return id, nil
}

func (c *CFG) RequireOpen(block BlockID) error {
	state, err := c.block(block)
	if err != nil {
		return err
	}
	if state.terminated {
		return fmt.Errorf("block %q already has a terminator", state.name)
	}
	return nil
}

func (c *CFG) Terminate(block BlockID) error {
	if err := c.RequireOpen(block); err != nil {
		return err
	}
	c.blocks[block-1].terminated = true
	return nil
}

func (c *CFG) ValidateTargets(from BlockID, targets ...BlockID) error {
	source, err := c.block(from)
	if err != nil {
		return err
	}
	if source.terminated {
		return fmt.Errorf("block %q already has a terminator", source.name)
	}
	for _, target := range targets {
		state, err := c.block(target)
		if err != nil {
			return err
		}
		if state.function != source.function {
			return fmt.Errorf("branch target %q belongs to a different function", state.name)
		}
	}
	return nil
}

func (c *CFG) TerminateWithTargets(from BlockID, targets ...BlockID) error {
	if err := c.ValidateTargets(from, targets...); err != nil {
		return err
	}
	c.blocks[from-1].terminated = true
	for _, target := range targets {
		if c.predecessors[target] == nil {
			c.predecessors[target] = make(map[BlockID]struct{})
		}
		c.predecessors[target][from] = struct{}{}
	}
	return nil
}

func (c *CFG) IsPredecessor(block, predecessor BlockID) bool {
	_, ok := c.predecessors[block][predecessor]
	return ok
}
func (c *CFG) PredecessorCount(block BlockID) int { return len(c.predecessors[block]) }

func (c *CFG) Finalize(function FunctionID) error {
	blocks := c.byFunction[function]
	if len(blocks) == 0 {
		return fmt.Errorf("function %d has no basic blocks", function)
	}
	for _, block := range blocks {
		state := c.blocks[block-1]
		if !state.terminated {
			return fmt.Errorf("block %q is not terminated", state.name)
		}
	}
	return nil
}

func (c *CFG) Function(block BlockID) (FunctionID, error) {
	state, err := c.block(block)
	if err != nil {
		return 0, err
	}
	return state.function, nil
}

// EntryBlock returns the first block appended to a function. Static local
// storage is inserted there even when its declaration occurs in a nested block.
func (c *CFG) EntryBlock(function FunctionID) (BlockID, error) {
	blocks := c.byFunction[function]
	if len(blocks) == 0 {
		return 0, fmt.Errorf("function %d has no entry block", function)
	}
	return blocks[0], nil
}

func (c *CFG) block(id BlockID) (*blockState, error) {
	if id == 0 || int(id) > len(c.blocks) {
		return nil, fmt.Errorf("unknown backend block %d", id)
	}
	return &c.blocks[id-1], nil
}
