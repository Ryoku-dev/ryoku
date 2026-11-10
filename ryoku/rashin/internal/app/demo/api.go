package demo

import (
	"fmt"
	"net"
	"strconv"

	"ryoku-rashin/internal/protocol"
)

func demoAPI(settings protocol.APISettings) protocol.APIInfo {
	return protocol.APIInfo{
		Enabled: settings.Enabled, Listening: settings.Enabled, Port: settings.Port,
		URL: "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(settings.Port)) + "/v1",
	}
}

// a key no service takes, counted so a new one reads differently
func (s *server) apiKey(rotate bool) protocol.APIKey {
	if rotate {
		s.data.apiKeys++
	}
	return protocol.APIKey{Key: fmt.Sprintf("rashin-demo%044d", s.data.apiKeys)}
}
