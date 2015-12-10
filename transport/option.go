package transport

type Option struct {
	// IgnoreReport: stop reporting when executing tasks.
	IR bool
}

func (me *Option) IgnoreReport() bool {
	return me.IR
}

func (me *Option) SetIgnoreReport(ignore bool) *Option {
	me.IR = ignore
	return me
}

func NewOption() *Option {
	return &Option{}
}
