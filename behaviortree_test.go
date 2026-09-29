package behaviortree_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/johndittmar/behaviortree"
)

type testState struct {
	syncs   int
	inits   int
	syncErr error
}

func (s *testState) Sync() error {
	s.syncs++
	return s.syncErr
}

type trueTask struct{}

func (task *trueTask) Run(*testState) behaviortree.Status { return behaviortree.Success }
func (task *trueTask) SetArgs(map[string]any)             {}
func (task *trueTask) Init(s *testState)                  { s.inits++ }

var registry = behaviortree.Registry[*testState]{
	"TrueTask": func() behaviortree.NodeTask[*testState] { return &trueTask{} },
}

type BehaviorTreeTestSuite struct {
	suite.Suite
	state *testState
}

func (suite *BehaviorTreeTestSuite) SetupTest() {
	suite.state = &testState{}
}

func (suite *BehaviorTreeTestSuite) readSetupFixture(file string) *behaviortree.Node[*testState] {
	sbtYaml, err := os.ReadFile(file)
	if err != nil {
		suite.FailNow("unable to read fixture file")
	}
	sbt, err := behaviortree.ReadBehaviorTree(sbtYaml)
	if err != nil {
		suite.FailNow("unable to unmarshal fixture file", err)
	}
	bt, err := registry.Build(sbt, suite.state)
	if err != nil {
		suite.FailNow("unable to build fixture tree", err)
	}
	suite.Nil(bt.GetParent())
	suite.Same(suite.state, bt.GetState())
	return bt
}

func TestBehaviorTreeSuite(t *testing.T) {
	suite.Run(t, new(BehaviorTreeTestSuite))
}

func (suite *BehaviorTreeTestSuite) TestRootTrueNode() {
	bt := suite.readSetupFixture("testdata/rootTrueNode.yaml")

	assert.Empty(suite.T(), bt.GetChildren())
	assert.Equal(suite.T(), behaviortree.Dormant, bt.GetStatus(), "root status should be dormant with no ticks ran yet")

	if assert.Equal(suite.T(), behaviortree.Task, bt.GetKind()) {
		assert.NotNil(suite.T(), bt.GetTask(), "node with kind task and nil task")
	}

	err := bt.Tick()
	assert.Nil(suite.T(), err, "a root task finishing is not an error")
	assert.Equal(suite.T(), behaviortree.Success, bt.GetStatus())

	bt.Reset()
	assert.Equal(suite.T(), behaviortree.Dormant, bt.GetStatus())
}

func (suite *BehaviorTreeTestSuite) TestSingleSequenceTrueChildNode() {
	bt := suite.readSetupFixture("testdata/sequenceTrue.yaml")

	if assert.Equal(suite.T(), behaviortree.Sequence, bt.GetKind()) {
		assert.Nil(suite.T(), bt.GetTask(), "node with kind sequence and non nil task")
	}

	err := bt.Tick()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), behaviortree.Success, bt.GetStatus())
}

func (suite *BehaviorTreeTestSuite) TestSequenceTrueChildNodes() {
	const numberOfChildren = 3
	bt := suite.readSetupFixture("testdata/sequenceThreeChildrenTrue.yaml")

	assert.Len(suite.T(), bt.GetChildren(), numberOfChildren)

	for range numberOfChildren - 1 {
		err := bt.Tick()
		assert.Nil(suite.T(), err)
	}

	assert.Equal(suite.T(), behaviortree.Running, bt.GetStatus())

	err := bt.Tick()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), behaviortree.Success, bt.GetStatus())
}

func (suite *BehaviorTreeTestSuite) TestSingleSelectorTrue() {
	bt := suite.readSetupFixture("testdata/singleSelectorTrue.yaml")

	assert.NotEmpty(suite.T(), bt.GetChildren())

	err := bt.Tick()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), behaviortree.Success, bt.GetStatus())
}

func (suite *BehaviorTreeTestSuite) TestSelectorTrueChildNodes() {
	const numberOfChildren = 3
	bt := suite.readSetupFixture("testdata/selectorThreeChildrenTrue.yaml")

	assert.Len(suite.T(), bt.GetChildren(), numberOfChildren)

	err := bt.Tick()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), behaviortree.Success, bt.GetStatus())
}

func (suite *BehaviorTreeTestSuite) TestRepeatorTrueChildNode() {
	bt := suite.readSetupFixture("testdata/repeatorTrue.yaml")

	for range 3 { // run a few times to make sure we're repeating
		err := bt.Tick()
		assert.Nil(suite.T(), err)

		assert.Equal(suite.T(), behaviortree.Success, bt.GetChildren()[0].GetStatus())
		assert.Equal(suite.T(), behaviortree.Success, bt.GetStatus())
	}
}

func (suite *BehaviorTreeTestSuite) TestInverterTrue() {
	bt := suite.readSetupFixture("testdata/inverterTrue.yaml")

	err := bt.Tick()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), behaviortree.Failure, bt.GetStatus())
}

func (suite *BehaviorTreeTestSuite) TestSyncOncePerTickAtRoot() {
	bt := suite.readSetupFixture("testdata/sequenceThreeChildrenTrue.yaml")

	for range 3 {
		suite.NoError(bt.Tick())
	}
	suite.Equal(3, suite.state.syncs)
}

func (suite *BehaviorTreeTestSuite) TestSyncErrorStopsTick() {
	bt := suite.readSetupFixture("testdata/rootTrueNode.yaml")
	suite.state.syncErr = errors.New("boom")

	suite.ErrorIs(bt.Tick(), suite.state.syncErr)
	suite.Equal(behaviortree.Dormant, bt.GetStatus())
}

func (suite *BehaviorTreeTestSuite) TestSetStateInitsEveryTask() {
	bt := suite.readSetupFixture("testdata/sequenceThreeChildrenTrue.yaml")
	suite.Equal(3, suite.state.inits)

	other := &testState{}
	bt.SetState(other)
	suite.Equal(3, other.inits)
	for _, node := range bt.GetDescendants() {
		suite.Same(other, node.GetState())
	}
}

func TestBuildInCode(t *testing.T) {
	root := behaviortree.NewNode[*testState](behaviortree.Sequence)
	leaf := behaviortree.NewNode[*testState](behaviortree.Task)
	assert.NoError(t, leaf.SetTask(&trueTask{}))
	root.AddChild(leaf)
	root.SetState(&testState{})

	assert.Error(t, root.SetTask(&trueTask{}), "only task nodes take a task")
	assert.NoError(t, root.Tick())
	assert.Equal(t, behaviortree.Success, root.GetStatus())
}

func TestTaskNodeWithoutTaskErrors(t *testing.T) {
	root := behaviortree.NewNode[*testState](behaviortree.Task)
	root.SetState(&testState{})
	assert.Error(t, root.Tick())
}

func TestBuildUnknownTask(t *testing.T) {
	sbt, err := behaviortree.ReadBehaviorTree([]byte("kind: Sequence\nchildren:\n  - kind: Task\n    task: Nope\n"))
	assert.NoError(t, err)

	_, err = registry.Build(sbt, &testState{})
	assert.ErrorContains(t, err, `unknown task "Nope"`)
}

func TestBuildTaskNodeWithoutTaskName(t *testing.T) {
	sbt, err := behaviortree.ReadBehaviorTree([]byte("kind: Task\n"))
	assert.NoError(t, err)

	_, err = registry.Build(sbt, &testState{})
	assert.Error(t, err)
}

type countTask struct{ runs *int }

func (task countTask) Run(*testState) behaviortree.Status {
	*task.runs++
	return behaviortree.Success
}
func (task countTask) SetArgs(map[string]any) {}

func TestSelectorWaitsOnRunningChild(t *testing.T) {
	root := behaviortree.NewNode[*testState](behaviortree.Selector)
	branch := behaviortree.NewNode[*testState](behaviortree.Sequence)
	for range 2 {
		leaf := behaviortree.NewNode[*testState](behaviortree.Task)
		assert.NoError(t, leaf.SetTask(&trueTask{}))
		branch.AddChild(leaf)
	}
	fallbackRuns := 0
	fallback := behaviortree.NewNode[*testState](behaviortree.Task)
	assert.NoError(t, fallback.SetTask(countTask{runs: &fallbackRuns}))
	root.AddChild(branch)
	root.AddChild(fallback)
	root.SetState(&testState{})

	assert.NoError(t, root.Tick())
	assert.Equal(t, behaviortree.Running, branch.GetStatus())
	assert.Equal(t, behaviortree.Running, root.GetStatus())
	assert.Zero(t, fallbackRuns, "fallback ran while the first branch was still running")

	assert.NoError(t, root.Tick())
	assert.Equal(t, behaviortree.Success, root.GetStatus())
	assert.Zero(t, fallbackRuns)
}
