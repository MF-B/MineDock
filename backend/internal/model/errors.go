package model

import "errors"

// ErrNameExists 表示实例名称已被占用。
var ErrNameExists = errors.New("instance name already exists")

// ErrInstanceRunning 表示实例正在运行，删除前必须先停止。
var ErrInstanceRunning = errors.New("instance is running, stop it before delete")
