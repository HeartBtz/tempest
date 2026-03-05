package handler

import (
	"net"
	"net/http"
)

type NetworkHandler struct{}

func NewNetworkHandler() *NetworkHandler {
	return &NetworkHandler{}
}

type InterfaceInfo struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Flags     string   `json:"flags"`
	MTU       int      `json:"mtu"`
}

func (h *NetworkHandler) ListInterfaces(w http.ResponseWriter, r *http.Request) {
	ifaces, err := net.Interfaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list interfaces: "+err.Error())
		return
	}

	result := make([]InterfaceInfo, 0, len(ifaces))
	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		info := InterfaceInfo{
			Name:  iface.Name,
			Flags: iface.Flags.String(),
			MTU:   iface.MTU,
		}

		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				info.Addresses = append(info.Addresses, addr.String())
			}
		}

		// Only include interfaces with addresses
		if len(info.Addresses) > 0 {
			result = append(result, info)
		}
	}

	writeJSON(w, http.StatusOK, result)
}
