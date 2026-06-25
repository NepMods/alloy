package hello_world

type HelloWorldLogger interface {
	LogFromHelloWorldAndGetARandomNumberBack(message string) int
}

var (
	EvenHelloWorldLogStarted = "hello_world.log.started"
	EvenHelloWorldLogDone    = "hello_world.log.done"
)

type HelloWoldData struct {
	message string
}
