package internal_test

// TODO: when we have better support for mocking shell commands, reinstate these tests

// type fakeShellExecutor struct {
// 	called []string
// 	err    error
// }

// func (f *fakeShellExecutor) ExecShell(containerName string) error {
// 	f.called = append(f.called, containerName)
// 	return f.err
// }

// func TestSessionShell_ByName(t *testing.T) {
// 	exec := &fakeShellExecutor{}
// 	cmd := SessionShellCmd{Name: "org/repo/work", exec: exec}
// 	err := cmd.Run()
// 	require.NoError(t, err)
// 	require.Equal(t, []string{"org-repo-work"}, exec.called)
// }

// func TestSessionShell_RequiresNameOrPick(t *testing.T) {
// 	cmd := SessionShellCmd{}
// 	err := cmd.Run()
// 	require.ErrorContains(t, err, "session name required")
// }

// func TestSessionShell_BubblesExecutorError(t *testing.T) {
// 	exec := &fakeShellExecutor{err: errors.New("boom")}
// 	cmd := SessionShellCmd{Name: "org/repo/work", exec: exec}
// 	err := cmd.Run()
// 	require.ErrorContains(t, err, "boom")
// }

// func TestSessionShell_PickNeedsTTY(t *testing.T) {
// 	cmd := SessionShellCmd{Pick: true}
// 	err := cmd.Run()
// 	require.ErrorContains(t, err, "requires an interactive TTY")
// }

// func TestSessionShell_InvalidSessionName(t *testing.T) {
// 	exec := &fakeShellExecutor{}
// 	cmd := SessionShellCmd{Name: "***", exec: exec}
// 	err := cmd.Run()
// 	require.ErrorContains(t, err, "unable to derive container name")
// }
