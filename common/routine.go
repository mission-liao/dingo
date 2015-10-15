package common

type RtControl struct {
	// a signal to tell go-routine to quit
	Quit chan int

	// a signal to let callers know the clear-up
	// is done
	Done chan int
}

func (me *RtControl) Close() {
	me.Quit <- 1
	close(me.Quit)

	<-me.Done
	close(me.Done)
}

func NewRtCtrl() *RtControl {
	return &RtControl{
		Quit: make(chan int, 1),
		Done: make(chan int, 1),
	}
}
