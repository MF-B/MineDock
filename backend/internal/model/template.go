package model

import "strings"

// GameTemplate 描述一个游戏服务器的完整技术配置模板。
type GameTemplate struct {
	Image     TemplateImage   `json:"image" yaml:"image"`
	Container ContainerConfig `json:"container" yaml:"container"`
	Params    []TemplateParam `json:"params" yaml:"params"`
}

// TemplateImage Docker 镜像配置。
type TemplateImage struct {
	Name string `json:"name" yaml:"name"`
	Tag  string `json:"tag" yaml:"tag"`
}

// FullImageRef 返回完整的镜像引用（name:tag）。
func (i TemplateImage) FullImageRef() string {
	name := strings.TrimSpace(i.Name)
	if name == "" {
		return ""
	}

	tag := strings.TrimSpace(i.Tag)
	if tag == "" {
		tag = "latest"
	}

	return name + ":" + tag
}

// ContainerConfig 容器运行配置。
type ContainerConfig struct {
	Ports       []PortMapping      `json:"ports" yaml:"ports"`
	Env         map[string]string  `json:"env" yaml:"env"`
	Volumes     []VolumeMount      `json:"volumes" yaml:"volumes"`
	Resources   *ResourceLimits    `json:"resources,omitempty" yaml:"resources"`
	Command     []string           `json:"command,omitempty" yaml:"command"`
	HealthCheck *HealthCheckConfig `json:"health_check,omitempty" yaml:"health_check"`
}

// PortMapping 端口映射配置。
type PortMapping struct {
	Host      int    `json:"host" yaml:"host"`
	Container int    `json:"container" yaml:"container"`
	Protocol  string `json:"protocol" yaml:"protocol"`
}

// VolumeMount 卷挂载配置。
type VolumeMount struct {
	Name          string `json:"name" yaml:"name"`
	ContainerPath string `json:"container_path" yaml:"container_path"`
	ReadOnly      bool   `json:"readonly" yaml:"readonly"`
}

// ResourceLimits 资源限制配置。
type ResourceLimits struct {
	Memory string  `json:"memory" yaml:"memory"`
	CPU    float64 `json:"cpu" yaml:"cpu"`
}

// HealthCheckConfig 健康检查配置。
type HealthCheckConfig struct {
	Test        []string `json:"test" yaml:"test"`
	Interval    string   `json:"interval" yaml:"interval"`
	Timeout     string   `json:"timeout" yaml:"timeout"`
	Retries     int      `json:"retries" yaml:"retries"`
	StartPeriod string   `json:"start_period" yaml:"start_period"`
}

// TemplateParam 用户可定制参数定义。
type TemplateParam struct {
	Key         string        `json:"key" yaml:"key"`
	Label       string        `json:"label" yaml:"label"`
	Description string        `json:"description" yaml:"description"`
	Type        string        `json:"type" yaml:"type"`
	Default     any           `json:"default" yaml:"default"`
	Options     []ParamOption `json:"options,omitempty" yaml:"options"`
	EnvVar      string        `json:"env_var,omitempty" yaml:"env_var"`
}

// ParamOption select 类型参数的可选项。
type ParamOption struct {
	Value string `json:"value" yaml:"value"`
	Label string `json:"label" yaml:"label"`
}
