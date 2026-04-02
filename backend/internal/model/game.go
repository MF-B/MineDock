package model

// Game 描述游戏目录中的一个条目（轻量展示信息）。
type Game struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon"`
}
