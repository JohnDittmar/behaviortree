# behaviortree

A small, generic behavior tree engine for Go.

```sh
go get github.com/johndittmar/behaviortree
```

- **Generic state.** A tree is a `Node[S]`. Every task in it receives the same
  state value `S`: your agent, blackboard, or whatever your tasks need.
- **Data-driven trees.** Describe trees in YAML and instantiate them with a
  `Registry` that maps task names to factories, or build them in code with
  `NewNode`, `AddChild` and `SetTask`.
- **Node kinds:** `Sequence`, `Selector`, `Repeator` (repeat), `Inverter`
  (negate), and `Task` leaves.

## Usage

```go
type Agent struct{ /* ... */ }

// Optional: called on the root at the start of every tick.
func (a *Agent) Sync() error { return nil }

type Wander struct{ speed int }

func (w *Wander) Run(a *Agent) behaviortree.Status { /* ... */ return behaviortree.Running }
func (w *Wander) SetArgs(args map[string]any)      { w.speed, _ = args["speed"].(int) }

registry := behaviortree.Registry[*Agent]{
	"Wander": func() behaviortree.NodeTask[*Agent] { return &Wander{} },
}

desc, err := behaviortree.ReadBehaviorTree([]byte(`
kind: Repeator
child:
  kind: Task
  task: Wander
  args: {speed: 3}
`))
root, err := registry.Build(desc, &Agent{})

// each frame:
err = root.Tick()
```

A node takes exactly one of `child`, `children` or `task` (with optional
`args`). Tasks may also implement `Init(S)`, which is called whenever the
tree's state is set.

See the [package example](example_test.go) for a complete program.

## Development

```sh
go test ./...
go generate ./...   # regenerates the enumer String/JSON/YAML methods
```

## License

[MIT](LICENSE)
