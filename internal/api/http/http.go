package http

type Config struct {
	Port           uint
	AgentPortRange PortRange
}

type PortRange struct {
	Start int
	End   int
}
