package auth

const (
	xaiClientID     = "b1a00492-073a-47ea-816f-4c329264a828"
	xaiAuthBase     = "https://auth.x.ai"
	xaiScope        = "openid profile email offline_access grok-cli:access api:access"
	xaiRedirectPort = 56121
)

func XAIFlow() FlowConfig {
	return FlowConfig{
		AuthorizeURL: xaiAuthBase + "/oauth2/authorize",
		TokenURL:     xaiAuthBase + "/oauth2/token",
		ClientID:     xaiClientID,
		Scope:        xaiScope,
		RedirectHost: "127.0.0.1",
		RedirectPort: xaiRedirectPort,
		RedirectPath: "/callback",
		IncludeNonce: true,
		ExtraParams: map[string]string{
			"plan":     "generic",
			"referrer": "mr-reviewer",
		},
	}
}

func XAIDeviceFlow() DeviceConfig {
	return DeviceConfig{
		DeviceURL: xaiAuthBase + "/oauth2/device/code",
		TokenURL:  xaiAuthBase + "/oauth2/token",
		ClientID:  xaiClientID,
		Scope:     xaiScope,
	}
}
