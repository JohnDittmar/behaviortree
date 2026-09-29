package behaviortree

import "fmt"

type Kind int

const (
	Sequence Kind = iota
	Selector
	Repeator
	Inverter
	Task
)

func (k *Kind) UnmarshalText(bs []byte) error {
	switch string(bs) {
	case "Sequence":
		*k = Sequence
	case "Selector":
		*k = Selector
	case "Repeator":
		*k = Repeator
	case "Inverter":
		*k = Inverter
	case "Task":
		*k = Task
	default:
		return fmt.Errorf("unknown kind %q", bs)
	}
	return nil
}

//go:generate go tool -modfile=tools/go.mod enumer -type=Kind
