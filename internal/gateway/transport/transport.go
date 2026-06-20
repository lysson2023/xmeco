package transport

type GatewayType string

const (
	TypeCustom      GatewayType = "custom"
	TypeTransparent GatewayType = "dtu"
)

// Transport abstracts how data is sent/received from a gateway
type Transport interface {
	Type() GatewayType
	GatewayID() string
	IsConnected() bool
	SendAndReceive(data []byte) ([]byte, error)
	Close() error
}
