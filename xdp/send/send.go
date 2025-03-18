package send

type sender interface {
	Send(nicName string, queueNum int) error
}
