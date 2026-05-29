package cli

type appConfig struct {
	JSON    bool
	Verbose bool
}

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }

func exitCode(code int, err error) error {
	return exitError{code: code, err: err}
}
