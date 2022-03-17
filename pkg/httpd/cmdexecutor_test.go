package httpd

import (
	"math/rand"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type MockFlusher struct{}

func (f MockFlusher) Flush() {}

type MockEncoder struct {
	Data        []interface{}
	ReturnError error
}

func (m *MockEncoder) Encode(v interface{}) error {
	m.Data = append(m.Data, v)
	return m.ReturnError
}

func getActiveCmd(require *require.Assertions) *exec.Cmd {
	cmd := exec.Command("echo", "0")
	err := cmd.Run()
	require.NoError(err)
	return cmd
}

func getChannels() (outputC chan Output, outputErrC chan error, outputDoneC chan interface{}) {
	outputC = make(chan Output, 4096)
	outputErrC = make(chan error)
	outputDoneC = make(chan interface{}, 2)
	return
}

func getMockEncoder() *ChunkedEncoder {
	mockEncoder := MockEncoder{}
	mockChunkedEncoder := ChunkedEncoder{
		Flusher: MockFlusher{},
		Encoder: &mockEncoder,
	}
	return &mockChunkedEncoder
}

func TestRun(t *testing.T) {
	require := require.New(t)

	execID := "execID"
	cmdExecutor := newCmdExecutor(execID)

	mockEncoder := MockEncoder{}
	mockChunkedEncoder := ChunkedEncoder{
		Flusher: MockFlusher{},
		Encoder: &mockEncoder,
	}
	outputC, outputErrC, outputDoneC := getChannels()

	// No active command.
	err := cmdExecutor.Run(outputC, outputDoneC, outputErrC, &mockChunkedEncoder)
	require.Error(err)

	// Test command executes normally.
	cmdExecutor.ActiveCmd = getActiveCmd(require)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = cmdExecutor.Run(outputC, outputDoneC, outputErrC, &mockChunkedEncoder)
		require.NoError(err)
	}()

	dummyData := Output{
		Msg: "hello",
	}
	outputC <- dummyData
	time.Sleep(100 * time.Millisecond)
	outputDoneC <- struct{}{}
	outputDoneC <- struct{}{}

	require.False(waitTimeout(&wg, time.Second))
	require.Len(mockEncoder.Data, 1)
	require.Equal(mockEncoder.Data[0], dummyData)

	// Test outputErrC.
	cmdExecutor.ActiveCmd = getActiveCmd(require)
	wg = sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := cmdExecutor.Run(outputC, outputDoneC, outputErrC, &mockChunkedEncoder)
		require.EqualError(err, "outputErrC")
	}()
	outputErrC <- errors.New("outputErrC")

	require.False(waitTimeout(&wg, time.Second))

	// Test encoder error.
	mockEncoder = MockEncoder{
		ReturnError: errors.New("encoderErr"),
	}
	mockChunkedEncoder = ChunkedEncoder{
		Flusher: MockFlusher{},
		Encoder: &mockEncoder,
	}
	cmdExecutor.ActiveCmd = getActiveCmd(require)
	wg = sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := cmdExecutor.Run(outputC, outputDoneC, outputErrC, &mockChunkedEncoder)
		require.EqualError(err, "encoderErr")
	}()
	outputC <- dummyData
	require.False(waitTimeout(&wg, time.Second))

	// Test command doesn't exit until outputDoneC is complete.
	cmdExecutor.ActiveCmd = getActiveCmd(require)
	wg = sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = cmdExecutor.Run(outputC, outputDoneC, outputErrC, &mockChunkedEncoder)
		require.NoError(err)
	}()
	outputDoneC <- struct{}{}

	require.True(waitTimeout(&wg, time.Second))
}

func TestGetCmd(t *testing.T) {
	require := require.New(t)
	cmd := GetCmd("my_python", []string{"-c", "print('helloworld')"}, map[string]interface{}{
		"MY_FOO": "bar",
	})
	require.Equal(cmd.Path, "my_python")
	require.Equal(cmd.Args, []string{"my_python", "-c", shellescape.Quote("print('helloworld')")})
	require.Contains(cmd.Env, "MY_FOO=bar")
}

func TestCmdExecutor(t *testing.T) {
	require := require.New(t)

	execID := "execID"
	cmdExecutor := newCmdExecutor(execID)
	cmd := exec.Command("echo", "0")

	// Check execute works
	_, _, err := cmdExecutor.Execute(cmd)
	require.NoError(err)

	// Check that running a command with a current command executes doesn't work
	_, _, err = cmdExecutor.Execute(cmd)
	require.Error(err)

	// Check that waiting works
	require.NoError(cmdExecutor.Wait())

	// Check that waiting doesn't work with a missing command
	require.Error(cmdExecutor.Wait())

	// Check that cancel works with a missing command
	require.NoError(cmdExecutor.Cancel())

	// Check that running an invalid path doesn't work
	cmdExecutor = newCmdExecutor(execID)
	cmd = exec.Command("invalid_path_command", "0")
	_, _, err = cmdExecutor.Execute(cmd)
	require.Error(err)
}

func TestCmdExecutorExecuteAndCancel(t *testing.T) {
	require := require.New(t)

	execID := "execID"
	cmdExecutor := newCmdExecutor(execID)

	outputC, outputErrC, outputDoneC := getChannels()
	stdoutPipe, stderrPipe, err := cmdExecutor.Execute(exec.Command("sleep", "100"))
	require.NoError(err)

	go WriteOutput(stdoutPipe, OutputTypeStdout, outputDoneC, outputC, outputErrC, cmdExecutor.ExecutionID)
	go WriteOutput(stderrPipe, OutputTypeStderr, outputDoneC, outputC, outputErrC, cmdExecutor.ExecutionID)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(cmdExecutor.Run(outputC, outputDoneC, outputErrC, getMockEncoder()))
		err := cmdExecutor.Wait()
		require.Error(err)
		require.Equal("signal: terminated", err.Error())
	}()
	require.NoError(cmdExecutor.Cancel())
	wg.Wait()
}

func TestCmdExecutorExecuteAndCancelSigkill(t *testing.T) {
	require := require.New(t)

	execID := "execID"
	cmdExecutor := newCmdExecutor(execID)

	// Check execute then cancel works with sigkill if we throw out sigterm
	checkCancelledProcessDuration = 100 * time.Millisecond
	sigtermCancelWaitDuration = 250 * time.Millisecond

	outputC, outputErrC, outputDoneC := getChannels()
	stdoutPipe, stderrPipe, err := cmdExecutor.Execute(exec.Command("sleep", "100"))
	require.NoError(err)

	go WriteOutput(stdoutPipe, OutputTypeStdout, outputDoneC, outputC, outputErrC, cmdExecutor.ExecutionID)
	go WriteOutput(stderrPipe, OutputTypeStderr, outputDoneC, outputC, outputErrC, cmdExecutor.ExecutionID)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(cmdExecutor.Cancel())
	}()
	// throw out first signal
	<-cmdExecutor.SignalC

	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(cmdExecutor.Run(outputC, outputDoneC, outputErrC, getMockEncoder()))
		err := cmdExecutor.Wait()
		require.Error(err)
		require.Equal("signal: killed", err.Error())
	}()
	wg.Wait()
}

func TestCmdExecutorExecuteAndCancelFailed(t *testing.T) {
	require := require.New(t)

	execID := "execID"
	cmdExecutor := newCmdExecutor(execID)

	// Check execute then cancel fails if we throw out sigterm and sigkill
	sigtermCancelWaitDuration = 250 * time.Millisecond
	sigkillCancelWaitDuration = 250 * time.Millisecond

	outputC, outputErrC, outputDoneC := getChannels()
	stdoutPipe, stderrPipe, err := cmdExecutor.Execute(exec.Command("sleep", "1"))
	require.NoError(err)

	go WriteOutput(stdoutPipe, OutputTypeStdout, outputDoneC, outputC, outputErrC, cmdExecutor.ExecutionID)
	go WriteOutput(stderrPipe, OutputTypeStderr, outputDoneC, outputC, outputErrC, cmdExecutor.ExecutionID)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Check that we arent about to cancel command
		require.Error(cmdExecutor.Cancel())
	}()
	// throw out first and second signal
	<-cmdExecutor.SignalC
	<-cmdExecutor.SignalC

	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(cmdExecutor.Run(outputC, outputDoneC, outputErrC, getMockEncoder()))
		err := cmdExecutor.Wait()
		require.NoError(err)
	}()
	wg.Wait()
}

func TestCmdExecutorManager(t *testing.T) {
	require := require.New(t)
	mutex := sync.Mutex{}
	manager := CmdExecutorManager{
		Executors: map[string]*CmdExecutor{},
		Mutex:     &mutex,
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	// Try and test that there aren't any deadlocks
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(ind int) {
			defer wg.Done()
			executor := manager.CreateExecutor()
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			require.NoError(manager.DeleteExecutor(&executor.ExecutionID))
		}(i)
	}
	wg.Wait()
	require.Len(manager.Executors, 0)

	// Missing exec id
	missingID := "missing"
	require.Error(manager.DeleteExecutor(&missingID))
	// Nothing to cancel
	require.Error(manager.DeleteExecutor(nil))
	require.Error(manager.DeleteExecutor(&missingID))

	executor := manager.CreateExecutor()
	stdoutPipe, stderrPipe, err := executor.Execute(exec.Command("sleep", "100"))
	require.NoError(err)

	// Test cancel with default exec id command
	outputC, outputErrC, outputDoneC := getChannels()

	go WriteOutput(stdoutPipe, OutputTypeStdout, outputDoneC, outputC, outputErrC, executor.ExecutionID)
	go WriteOutput(stderrPipe, OutputTypeStderr, outputDoneC, outputC, outputErrC, executor.ExecutionID)

	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(executor.Run(outputC, outputDoneC, outputErrC, getMockEncoder()))
		err := executor.Wait()
		require.Error(err)
		require.Equal("signal: terminated", err.Error())
	}()

	require.Len(manager.Executors, 1)
	require.NoError(manager.DeleteExecutor(nil))
	require.Len(manager.Executors, 0)
	wg.Wait()

	// Test cancel while specifying exec id
	executor = manager.CreateExecutor()
	stdoutPipe, stderrPipe, err = executor.Execute(exec.Command("sleep", "100"))
	require.NoError(err)

	go WriteOutput(stdoutPipe, OutputTypeStdout, outputDoneC, outputC, outputErrC, executor.ExecutionID)
	go WriteOutput(stderrPipe, OutputTypeStderr, outputDoneC, outputC, outputErrC, executor.ExecutionID)

	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(executor.Run(outputC, outputDoneC, outputErrC, getMockEncoder()))
		err := executor.Wait()
		require.Error(err)
		require.Equal("signal: terminated", err.Error())
	}()

	require.Len(manager.Executors, 1)
	require.NoError(manager.DeleteExecutor(&executor.ExecutionID))
	require.Len(manager.Executors, 0)
	wg.Wait()
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}
