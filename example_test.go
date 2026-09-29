package behaviortree_test

import (
	"fmt"

	"github.com/johndittmar/behaviortree"
)

// guard is the shared state every task in the tree sees.
type guard struct {
	intruders int
	log       []string
}

// Sync runs at the root before each tick; use it to refresh senses.
func (g *guard) Sync() error { return nil }

type spot struct{}

func (spot) Run(g *guard) behaviortree.Status {
	if g.intruders > 0 {
		return behaviortree.Success
	}
	return behaviortree.Failure
}
func (spot) SetArgs(map[string]any) {}

type say struct{ line string }

func (s *say) Run(g *guard) behaviortree.Status {
	g.log = append(g.log, s.line)
	return behaviortree.Success
}
func (s *say) SetArgs(args map[string]any) { s.line, _ = args["line"].(string) }

func Example() {
	tree, err := behaviortree.ReadBehaviorTree([]byte(`
kind: Selector
children:
  - kind: Sequence
    children:
      - kind: Task
        task: Spot
      - kind: Task
        task: Say
        args: {line: "Halt!"}
  - kind: Task
    task: Say
    args: {line: "All quiet."}
`))
	if err != nil {
		panic(err)
	}

	registry := behaviortree.Registry[*guard]{
		"Spot": func() behaviortree.NodeTask[*guard] { return spot{} },
		"Say":  func() behaviortree.NodeTask[*guard] { return &say{} },
	}

	g := &guard{}
	root, err := registry.Build(tree, g)
	if err != nil {
		panic(err)
	}
	for root.GetStatus() != behaviortree.Success && root.GetStatus() != behaviortree.Failure {
		if err := root.Tick(); err != nil {
			panic(err)
		}
	}
	fmt.Println(g.log, root.GetStatus())
	// Output: [All quiet.] Success
}
