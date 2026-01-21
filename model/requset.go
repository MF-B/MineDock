package model

// CreateRequest 定义了前端必须要传过来的参数
type CreateRequest struct {
	Name     string            `json:"name"` // 容器名字，比如 "create-server"
	Port     string            `json:"port"` // 端口，比如 "25565"
	Env      map[string]string `json:"env"`
	DataPath string            `json:"dataPath"` // 重点！你的存档在宿主机的哪个路径
	Image    string            `json:"image"`
}
