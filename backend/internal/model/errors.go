package model

import "errors"

// ErrNameExists 表示实例名称已被占用。
var ErrNameExists = errors.New("instance name already exists")

// ErrInstanceRunning 表示实例正在运行，删除前必须先停止。
var ErrInstanceRunning = errors.New("instance is running, stop it before delete")

// ErrGameNotFound 表示请求的游戏 ID 不在目录中。
var ErrGameNotFound = errors.New("game not found")

// ErrTemplateNotFound 表示请求的模板文件不存在。
var ErrTemplateNotFound = errors.New("template not found")

// ErrTemplateInvalid 表示模板内容不合法。
var ErrTemplateInvalid = errors.New("invalid template")

// ErrInvalidParams 表示传入了不受模板定义约束的参数。
var ErrInvalidParams = errors.New("invalid params")

// ErrInvalidResourceLimits 表示传入了非法的 CPU 或内存资源限制。
var ErrInvalidResourceLimits = errors.New("invalid resource limits")

// ErrContainerNotStopped 表示容器必须先停止，才能执行配置更新。
var ErrContainerNotStopped = errors.New("container must be stopped to update config")
