package behaviortree

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// StatelessBehaviorNode is the serializable description of a tree. It names
// tasks rather than holding them, so one description can be instantiated many
// times with Registry.Build.
type StatelessBehaviorNode struct {
	Kind     Kind
	Children []*StatelessBehaviorNode
	Task     string
	Args     map[string]any
}

func (sbn *StatelessBehaviorNode) AddChild(childNode *StatelessBehaviorNode) {
	if sbn.Children == nil {
		sbn.Children = make([]*StatelessBehaviorNode, 0)
	}
	sbn.Children = append(sbn.Children, childNode)
}

// UnmarshalYAML accepts exactly one of `child`, `children` or `task` (with
// optional `args`) alongside `kind`.
func (sbn *StatelessBehaviorNode) UnmarshalYAML(value *yaml.Node) error {
	type Node struct {
		Kind     Kind                     `yaml:"kind"`
		Child    *StatelessBehaviorNode   `yaml:"child,omitempty"`
		Children []*StatelessBehaviorNode `yaml:"children,omitempty"`
		Task     string                   `yaml:"task,omitempty"`
		Args     map[string]any           `yaml:"args,omitempty"`
	}
	var node Node
	if err := value.Decode(&node); err != nil {
		return err
	}
	*sbn = StatelessBehaviorNode{Kind: node.Kind}

	switch {
	case node.Child != nil && node.Children == nil && len(node.Task) == 0:
		sbn.AddChild(node.Child)
	case node.Children != nil && node.Child == nil && len(node.Task) == 0:
		for _, child := range node.Children {
			sbn.AddChild(child)
		}
	case len(node.Task) > 0 && node.Children == nil && node.Child == nil:
		sbn.Task = node.Task
		sbn.Args = node.Args
	case node.Child != nil || node.Children != nil || len(node.Task) > 0:
		return fmt.Errorf("line %d: a %s node takes only one of child, children or task", value.Line, node.Kind)
	}
	return nil
}

// TaskFactory returns a fresh NodeTask each time it is called.
type TaskFactory[S any] func() NodeTask[S]

// Registry maps the task names used in a StatelessBehaviorNode to the
// factories that create them.
type Registry[S any] map[string]TaskFactory[S]

func (r Registry[S]) Register(name string, factory TaskFactory[S]) {
	r[name] = factory
}

// Build instantiates the described tree, creating a new task for every Task
// node, and sets state on the whole tree.
func (r Registry[S]) Build(sbn *StatelessBehaviorNode, state S) (*Node[S], error) {
	root, err := r.build(sbn)
	if err != nil {
		return nil, err
	}
	root.SetState(state)
	return root, nil
}

func (r Registry[S]) build(sbn *StatelessBehaviorNode) (*Node[S], error) {
	if sbn == nil {
		return nil, errors.New("nil behavior node")
	}
	bt := NewNode[S](sbn.Kind)
	if sbn.Kind == Task {
		if len(sbn.Task) == 0 {
			return nil, errors.New("task node with no task name")
		}
		factory, ok := r[sbn.Task]
		if !ok {
			return nil, fmt.Errorf("unknown task %q", sbn.Task)
		}
		task := factory()
		if sbn.Args != nil {
			task.SetArgs(sbn.Args)
		}
		bt.task = task
	} else if len(sbn.Task) > 0 {
		return nil, fmt.Errorf("task %q set on a %s node", sbn.Task, sbn.Kind)
	}
	for _, child := range sbn.Children {
		childNode, err := r.build(child)
		if err != nil {
			return nil, err
		}
		bt.AddChild(childNode)
	}
	return bt, nil
}

func ReadBehaviorTree(b []byte) (*StatelessBehaviorNode, error) {
	f := &StatelessBehaviorNode{}
	if err := yaml.Unmarshal(b, f); err != nil {
		return nil, err
	}
	return f, nil
}
