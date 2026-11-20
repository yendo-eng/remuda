package smolvm

// Mock is an in-memory Client for tests.
type Mock struct {
	// Machines maps existing machine names to whether they run.
	Machines map[string]bool

	// Execs records Exec calls.
	Execs []struct {
		Machine string
		Command string
	}

	// Deleted records machines Delete removed.
	Deleted []string
}

func (m *Mock) MachineRunning(name string) (bool, error) {
	running, ok := m.Machines[name]
	if !ok {
		return false, ErrMachineNotFound
	}
	return running, nil
}

func (m *Mock) Exec(name string, command string) error {
	m.Execs = append(m.Execs, struct {
		Machine string
		Command string
	}{name, command})
	return nil
}

func (m *Mock) Delete(name string) error {
	if _, ok := m.Machines[name]; !ok {
		return ErrMachineNotFound
	}
	delete(m.Machines, name)
	m.Deleted = append(m.Deleted, name)
	return nil
}
