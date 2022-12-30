package v1

func NeighborContains(slice []Neighbor, target Neighbor) (int, bool) {
	for i, iterm := range slice {
		if iterm.HardwareAddr == target.HardwareAddr {
			return i, true
		}
	}
	return 0, false
}
