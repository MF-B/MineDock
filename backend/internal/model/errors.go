package model

import "errors"

// ErrNameExists indicates that an instance name is already in use.
var ErrNameExists = errors.New("instance name already exists")

// ErrInstanceRunning indicates the instance is running and must be stopped before delete.
var ErrInstanceRunning = errors.New("instance is running, stop it before delete")
