package counter

type CountManager interface {
	Add(count int) int
}

var (
	EvenCounterStarted = "count.started"
	EvenCounterDone    = "count.done"
)

type CounterData struct {
	message string
}
