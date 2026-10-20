package runtime

import "fmt"

// BlockReference is a block as `self.name` and `super` give it, a port of
// jinja2.runtime.BlockReference: calling it renders the block, and its
// super attribute is the parent block's reference.
type BlockReference struct {
	// Render renders the block.
	Render func() (any, error)
	// Super returns the parent block's reference, or an Undefined.
	Super func() any
}

// String is Python's repr.
func (b *BlockReference) String() string {
	return fmt.Sprintf("<jinja2.runtime.BlockReference object at %p>", b)
}

// Get implements the super attribute.
func (b *BlockReference) Get(attr string) (any, bool) {
	if attr == "super" {
		return b.Super(), true
	}
	return nil, false
}
