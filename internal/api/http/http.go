package http

type Config struct {
	Port           uint      `mapstructure:"port"`
	AgentPortRange PortRange `mapstructure:"agent_port_range"`
}

type PortRange struct {
	Start int `mapstructure:"start"`
	End   int `mapstructure:"end"`
}
