package behaviortree

type Status int

const (
	Dormant Status = iota - 1
	Running
	Failure
	Success
)

//go:generate go tool -modfile=tools/go.mod enumer -type=Status
