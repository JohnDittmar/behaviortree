// Package behaviortree is a small, generic behavior tree engine.
//
// A tree is made of Sequence, Selector, Repeator and Inverter composite nodes
// whose leaves are Task nodes. Every node in a tree shares a single state value
// of type S, which is handed to each task when it runs. Trees can be built in
// code with NewNode/AddChild/SetTask, or described in YAML as a
// StatelessBehaviorNode and instantiated with a Registry.
package behaviortree

import (
	"errors"
	"log/slog"
)

// NodeTask is the unit of work at a Task leaf. Run is called on each tick
// until it returns a terminal Status (Failure or Success).
type NodeTask[S any] interface {
	Run(state S) Status
	SetArgs(args map[string]any)
}

// NodeTaskIniter is optionally implemented by a NodeTask that needs to prepare
// itself whenever the tree's state is set.
type NodeTaskIniter[S any] interface {
	Init(state S)
}

// Syncer is optionally implemented by a tree's state. When it is, the root
// node calls Sync at the start of every tick, before any task runs.
type Syncer interface {
	Sync() error
}

type Node[S any] struct {
	status   Status
	kind     Kind
	parent   *Node[S]
	children []*Node[S]
	state    S
	task     NodeTask[S]
}

// NewNode returns a dormant node of the given kind.
func NewNode[S any](kind Kind) *Node[S] {
	return &Node[S]{kind: kind, status: Dormant}
}

func (bn *Node[S]) GetStatus() Status {
	return bn.status
}

func (bn *Node[S]) GetKind() Kind {
	return bn.kind
}

func (bn *Node[S]) GetParent() *Node[S] {
	return bn.parent
}

func (bn *Node[S]) GetChildren() []*Node[S] {
	return bn.children
}

func (bn *Node[S]) GetDescendants() []*Node[S] {
	descendants := make([]*Node[S], 0)
	bn.getDescendants(&descendants)
	return descendants

}

func (bn *Node[S]) getDescendants(ret *[]*Node[S]) {
	*ret = append(*ret, bn)
	for _, child := range bn.children {
		child.getDescendants(ret)
	}
}

func (bn *Node[S]) GetState() S {
	return bn.state
}

func (bn *Node[S]) GetTask() NodeTask[S] {
	return bn.task
}

func (bn *Node[S]) AddChild(childNode *Node[S]) {
	if bn.children == nil {
		bn.children = make([]*Node[S], 0)
	}
	bn.children = append(bn.children, childNode)
	childNode.parent = bn
	childNode.state = bn.state
}

// SetState sets the state on this node and all of its descendants, calling
// Init on any task that implements NodeTaskIniter.
func (bn *Node[S]) SetState(state S) {
	bn.state = state
	if initer, ok := bn.task.(NodeTaskIniter[S]); ok {
		initer.Init(state)
	}
	for _, child := range bn.children {
		child.SetState(state)
	}
}

func (bn *Node[S]) Tick() error {
	if bn == nil {
		slog.Error("tick on nil behaviortree.Node")
		return nil
	}
	if bn.parent == nil {
		if syncer, ok := any(bn.state).(Syncer); ok {
			if err := syncer.Sync(); err != nil {
				return err
			}
		}
	}
	switch bn.kind {
	case Sequence:
		if len(bn.children) == 0 {
			bn.status = Failure
			slog.Error("tried running a sequence tick with no children")
		} else {
			bn.status = Running
			allChildrenSuccessfull := true
			for i, child := range bn.children {
				if child.status <= Running {
					err := child.Tick()
					if err != nil {
						slog.Error("child tick returned an error", slog.Any("err", err))
						return err
					}
					// Since we're a sequence, if child fails, parent node fails
					if child.status == Failure {
						bn.status = Failure
						break
					}
					if child.status < Success {
						allChildrenSuccessfull = false
					}
					// If we're at the end and everything was successful, set us a successful
					if i == len(bn.children)-1 && allChildrenSuccessfull {
						bn.status = Success
					}
					break // only one sequential tick at a time
				}
			}
		}
	case Selector:
		if len(bn.children) == 0 {
			bn.status = Failure
			slog.Error("tried running a selector tick with no children")
		} else {
			bn.status = Running
			for i, child := range bn.children {
				if child.status <= Running {
					err := child.Tick()
					if err != nil {
						slog.Error("child tick returned an error", slog.Any("err", err))
						return err
					}
					if child.status == Success {
						bn.status = Success
						break
					}
					if child.status == Running {
						break // only move on to the next child once this one fails
					}
					if i+1 == len(bn.children) {
						bn.status = Failure
					}
				}
			}
		}
	case Inverter:
		if len(bn.children) == 0 {
			bn.status = Failure
			return errors.New("tried running a decorator tick with no children")
		} else if len(bn.children) > 1 {
			bn.status = Failure
			return errors.New("tried running decorator tick with more than one child")
		}
		bn.status = Running
		child := bn.children[0]
		err := child.Tick()
		if err != nil {
			return err
		}
	case Repeator:
		if len(bn.children) == 0 {
			bn.status = Failure
			return errors.New("tried running a decorator tick with no children")
		} else if len(bn.children) > 1 {
			bn.status = Failure
			return errors.New("tried running decorator tick with more than one child")
		}
		if bn.status == Success { // Success is trigger value for reset on Repeators
			if err := bn.Reset(); err != nil {
				return err
			}
		}
		bn.status = Running
		child := bn.children[0]
		err := child.Tick()
		if err != nil {
			return err
		}
	case Task:
		if bn.task == nil {
			bn.status = Failure
			return errors.New("tried running a task tick with no task set")
		}
		switch bn.status {
		case Running, Dormant:
			bn.status = bn.task.Run(bn.state)
			if bn.status >= Failure {
				err := bn.notifyParent()
				if err != nil {
					return err
				}
			}
		case Failure, Success:
			slog.Warn("tried to tick on an already terminal node")
		}
	}
	return nil
}

func (bn *Node[S]) SetTask(task NodeTask[S]) error {
	if bn.kind != Task {
		return errors.New("trying to set task to a non task behavior node")
	}
	bn.task = task
	return nil

}

func (bn *Node[S]) notifyParent() error {
	if bn.status < Failure {
		return errors.New("tried to notify parent of non terminal status")
	}
	if bn.parent == nil {
		return nil // the root has no one to notify
	}
	switch bn.parent.kind {
	case Sequence:
		if bn.status == Failure || bn == bn.parent.children[len(bn.parent.children)-1] {
			bn.parent.status = bn.status
			return bn.parent.notifyParent()
		}
	case Selector:
		if bn.status == Success || bn == bn.parent.children[len(bn.parent.children)-1] {
			bn.parent.status = bn.status
			return bn.parent.notifyParent()
		}
	case Repeator:
		bn.parent.status = Success
	case Inverter:
		if bn.status == Failure {
			bn.parent.status = Success
		} else { // result == Success
			bn.parent.status = Failure
		}
		return bn.parent.notifyParent()
	}
	return nil
}

func (bn *Node[S]) Reset() error {
	bn.status = Dormant
	for _, child := range bn.children {
		err := child.Reset()
		if err != nil {
			return err
		}
	}
	return nil
}
