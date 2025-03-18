package receive

type receiver interface {
	Receive(nicName string, msgChan <-chan []byte) error
}
