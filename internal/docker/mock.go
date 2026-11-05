package docker

type Mock struct {
	// Whether the running check should succeed or not
	Running bool

	// ContainerRunning values (keyed by container name). When absent, defaults to ErrContainerNotFound.
	RunningContainers map[string]bool

	// docker exec commands that were passed in.
	Execs []struct {
		Container string
		Command   string
	}
}

func (m Mock) CheckRunning() error {
	if !m.Running {
		return ErrNotRunning
	}

	return nil
}

func (m Mock) ContainerRunning(container string) (bool, error) {
	if m.RunningContainers != nil {
		if running, ok := m.RunningContainers[container]; ok {
			return running, nil
		}
	}
	return false, ErrContainerNotFound
}

func (m *Mock) Exec(container string, command string) error {
	m.Execs = append(m.Execs, struct {
		Container string
		Command   string
	}{
		Container: container,
		Command:   command,
	})
	return nil
}
