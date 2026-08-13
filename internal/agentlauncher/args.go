package agentlauncher

import (
	"strings"

	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

type launchArgument struct {
	value string
	shell string
}

type launch struct {
	executable string
	arguments  []launchArgument
}

func (l launch) command() string {
	var command strings.Builder
	command.WriteString(l.executable)
	for _, argument := range l.arguments {
		command.WriteByte(' ')
		command.WriteString(argument.shell)
	}
	return command.String()
}

func (l launch) argv() []string {
	args := make([]string, 0, len(l.arguments))
	for _, argument := range l.arguments {
		args = append(args, argument.value)
	}
	return args
}

func (l *launch) raw(values ...string) {
	for _, value := range values {
		l.arguments = append(l.arguments, launchArgument{value: value, shell: value})
	}
}

func (l *launch) quoted(values ...string) {
	for _, value := range values {
		l.arguments = append(l.arguments, launchArgument{value: value, shell: shellutil.SingleQuote(value)})
	}
}

func (l *launch) rendered(value, shell string) {
	l.arguments = append(l.arguments, launchArgument{value: value, shell: shell})
}
