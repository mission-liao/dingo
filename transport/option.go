package transport

type Option struct {
	IR bool
}

//
// getter
//

func (me *Option) IgnoreReport() bool {
	return me.IR
}

//
// setter
//

func (me *Option) SetIgnoreReport(ignore bool) *Option {
	me.IR = ignore
	return me
}

func NewOption() *Option {
	return &Option{}
}
