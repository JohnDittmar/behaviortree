package behaviortree_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/johndittmar/behaviortree"
)

func TestUnmarshal(t *testing.T) {
	out := `
kind: Repeator
child:
  kind: Selector
  children:
    - kind: Sequence
      children:
        - kind: Task
          task: MoveTowards
          args:
            rangeStop: 3
`
	var sbt behaviortree.StatelessBehaviorNode

	err := yaml.Unmarshal([]byte(out), &sbt)
	assert.Nil(t, err, "this should be nil")
}

func TestUnmarshalErrors(t *testing.T) {
	for name, in := range map[string]string{
		"unknown kind":       "kind: Parallel\n",
		"child and task":     "kind: Inverter\ntask: TrueTask\nchild:\n  kind: Task\n  task: TrueTask\n",
		"child and children": "kind: Sequence\nchild:\n  kind: Task\n  task: TrueTask\nchildren: []\n",
		"not a mapping":      "- kind: Task\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := behaviortree.ReadBehaviorTree([]byte(in))
			assert.Error(t, err)
		})
	}
}

func TestUnmarshalArgs(t *testing.T) {
	sbt, err := behaviortree.ReadBehaviorTree([]byte("kind: Task\ntask: Spawner\nargs:\n  spawnCell: Clip\n  interval: 2\n"))
	assert.NoError(t, err)
	assert.Equal(t, "Spawner", sbt.Task)
	assert.Equal(t, map[string]any{"spawnCell": "Clip", "interval": 2}, sbt.Args)
}
