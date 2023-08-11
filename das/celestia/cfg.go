package celestia

type DAConfig struct {
	Enable      bool   `koanf:"enable"`
	Rpc         string `koanf:"rpc"`
	NamespaceId string `koanf:"namespace-id"`
	AuthToken   string `koanf:"auth-token"`
}

func NewDAConfig(l1NodeUrl, sequencerInboxAddress, rpc, ns string, l1ConnectionAttemps int) (*DAConfig, error) {
	return &DAConfig{
		Enable:      true,
		Rpc:         rpc,
		NamespaceId: ns,
	}, nil
}
