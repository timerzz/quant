package pusher

type Pusher interface {
	Push(msg string)
}
