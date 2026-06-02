package internal

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIOHelperMethods(t *testing.T) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	io := IO{
		Out: &outBuf,
		Err: &errBuf,
	}

	io.Outf("hello %s", "world")
	require.Equal(t, "hello world", outBuf.String())
	outBuf.Reset()

	io.Outln("line")
	require.Equal(t, "line\n", outBuf.String())
	outBuf.Reset()

	io.Outln("foo", "bar")
	require.Equal(t, "foo bar\n", outBuf.String())
	outBuf.Reset()

	io.OutWrite("foo", "bar")
	require.Equal(t, "foobar", outBuf.String())
	outBuf.Reset()

	io.Errf("error %d", 42)
	require.Equal(t, "error 42", errBuf.String())
	errBuf.Reset()

	io.Errln("boom")
	require.Equal(t, "boom\n", errBuf.String())
	errBuf.Reset()

	io.ErrWrite("z", "y")
	require.Equal(t, "zy", errBuf.String())
}
