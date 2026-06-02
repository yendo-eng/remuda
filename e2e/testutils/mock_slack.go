package testutils

type MockSlack struct {
	// url => thread messages
	Threads map[string]string
}

func (m MockSlack) GetThread(threadURL string) (string, error) {
	if msg, ok := m.Threads[threadURL]; ok {
		return msg, nil
	}
	return "", nil
}
