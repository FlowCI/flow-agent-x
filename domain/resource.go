package domain

type Resource struct {
	Cpu         int    `json:"cpu"`
	TotalMemory uint64 `json:"totalMemory"`
	FreeMemory  uint64 `json:"freeMemory"`
	TotalDisk   uint64   `json:"totalDisk"`
	FreeDisk    uint64   `json:"freeDisk"`
}
