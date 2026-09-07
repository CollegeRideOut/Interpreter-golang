package comparator

import (
	"github.com/Xanonymous-GitHub/gumtree-go/ast"
	"log/slog"
)

type Comparator interface {
	Compare() []Mapping
}

// Mapping identifies a node correspondence between the source and target ASTs.
type Mapping struct {
	Source *ast.Node
	Target *ast.Node
}

type comparator struct {
	tree1, tree2       *ast.AST
	list1, list2       HeightIndexedPriorityList
	candidateMappings  mappingsType
	uniqueMappings     mappingsType
	minDice            float64
	minHeight, maxSize int
	logger             slog.Logger
}

// Compare computes the GumTree node mappings between the two ASTs.
func (c *comparator) Compare() []Mapping {
	c.topDown()

	result := make([]Mapping, 0, len(c.uniqueMappings))
	for _, mapping := range c.uniqueMappings {
		result = append(result, Mapping{
			Source: mapping.Left(),
			Target: mapping.Right(),
		})
	}
	return result
}

func NewComparator(
	tree1, tree2 *ast.AST,
	minHeight, maxSize int,
	minDice float64,
	logger slog.Logger,
) Comparator {
	if tree1 == nil || tree2 == nil {
		panic("trees cannot be nil")
	}
	if minHeight < 0 {
		panic("minHeight cannot be negative")
	}

	return &comparator{
		tree1:             tree1,
		tree2:             tree2,
		list1:             NewHeightIndexedPriorityList(logger),
		list2:             NewHeightIndexedPriorityList(logger),
		candidateMappings: make(mappingsType, 0),
		uniqueMappings:    make(mappingsType, 0),
		minDice:           minDice,
		minHeight:         minHeight,
		maxSize:           maxSize,
	}
}
