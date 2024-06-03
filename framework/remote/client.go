package remote

type Client interface {
	Run() error
	SendMsg(string, []byte) error
	Close() error
}
