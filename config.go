//revive:disable:package-comments
package connectserver

type configuration struct {
	MaxRecvMsgSize int `env:"MAX_RECV_MSG_SIZE" envDefault:"4194304"`
	MaxSendMsgSize int `env:"MAX_SEND_MSG_SIZE" envDefault:"4194304"`
}
